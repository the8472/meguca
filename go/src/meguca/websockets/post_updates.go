package websockets

import (
	"encoding/json"
	"errors"
	"meguca/common"
	"meguca/config"
	"meguca/db"
	"meguca/parser"
	"meguca/util"
	"unicode/utf8"
)

var (
	errNoPostOpen          = errors.New("no post open")
	errEmptyPost           = errors.New("post body empty")
	errTooManyLines        = errors.New("too many lines in post body")
	errInvalidSpliceCoords = errors.New("invalid splice coordinates")
	errSpliceTooLong       = errors.New("splice text too long")
	errSpliceNOOP          = errors.New("splice NOOP")
	errTextOnly            = errors.New("text only board")
	errHasImage            = errors.New("post already has image")
)

// Like spliceRequest, but with a string Text field. Used for internal
// conversions between []rune and string.
type spliceRequestString struct {
	spliceCoords
	Text string `json:"text"`
}

// Common part of a splice request and a splice response
type spliceCoords struct {
	Start int `json:"start"`
	Len   int `json:"len"`
}

// Response to a spliceRequest. Sent to all listening clients.
type spliceMessage struct {
	ID uint64 `json:"id"`
	spliceRequestString
}

// Request or to replace the current line's text starting at an exact position
// in the current line
type spliceRequest struct {
	spliceCoords
	Text []rune
}

// Custom unmarshaling of string -> []rune
func (s *spliceRequest) UnmarshalJSON(buf []byte) error {
	var tmp spliceRequestString
	if err := json.Unmarshal(buf, &tmp); err != nil {
		return err
	}
	*s = spliceRequest{
		spliceCoords: tmp.spliceCoords,
		Text:         []rune(tmp.Text),
	}
	return nil
}

// Custom marshaling of []rune -> string
func (s spliceRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(spliceRequestString{
		spliceCoords: s.spliceCoords,
		Text:         string(s.Text),
	})
}

// Append a rune to the body of the open post
func (c *Client) appendRune(data []byte) (err error) {
	has, err := c.hasPost()
	switch {
	case err != nil:
		return
	case !has:
		return
	case c.post.len+1 > common.MaxLenBody:
		return common.ErrBodyTooLong
	}

	var char rune
	err = decodeMessage(data, &char)
	switch {
	case err != nil:
		return
	case char == 0:
		return common.ErrContainsNull
	case char == '\n':
		c.post.lines++
		if c.post.lines > common.MaxLinesBody {
			return errTooManyLines
		}
	}

	msg, err := common.EncodeMessage(
		common.MessageAppend,
		[2]uint64{c.post.id, uint64(char)},
	)
	if err != nil {
		return
	}

	c.post.body = append(c.post.body, string(char)...)
	c.post.len++
	return c.updateBody(msg)
}

// Send message to thread update feed and writes the open post's buffer to the
// embedded database. Requires locking of c.openPost.
func (c *Client) updateBody(msg []byte) error {
	c.feed.SetOpenBody(c.post.id, c.post.body, msg)
	return db.SetOpenBody(c.post.id, c.post.body)
}

// Remove one character from the end of the line in the open post
func (c *Client) backspace() error {
	has, err := c.hasPost()
	switch {
	case err != nil:
		return err
	case !has:
		return nil
	case c.post.len == 0:
		return errEmptyPost
	}

	msg, err := common.EncodeMessage(common.MessageBackspace, c.post.id)
	if err != nil {
		return err
	}

	r, lastRuneLen := utf8.DecodeLastRune(c.post.body)
	c.post.body = c.post.body[:len(c.post.body)-lastRuneLen]
	if r == '\n' {
		c.post.lines--
	}
	c.post.len--

	return c.updateBody(msg)
}

// Close an open post and parse the last line, if needed.
func (c *Client) closePost() error {
	if c.post.id == 0 {
		return errNoPostOpen
	}

	var (
		links [][2]uint64
		com   []common.Command
		err   error
	)
	if c.post.len != 0 {
		links, com, err = parser.ParseBody(c.post.body, c.post.board)
		if err != nil {
			return err
		}
	}

	err = db.ClosePost(c.post.id, c.post.op, string(c.post.body), links, com)
	if err != nil {
		return err
	}
	c.post = openPost{}
	return nil
}

// Splice the text in the open post. This call is also used for text pastes.
func (c *Client) spliceText(data []byte) error {
	if has, err := c.hasPost(); err != nil {
		return err
	} else if !has {
		return nil
	}

	// Decode and validate
	var req spliceRequest
	err := decodeMessage(data, &req)
	switch {
	case err != nil:
		return err
	case req.Start < 0, req.Len < -1, req.Start+req.Len > c.post.len:
		return errInvalidSpliceCoords
	case req.Len == 0 && len(req.Text) == 0:
		return errSpliceNOOP // This does nothing. Client-side error.
	case len(req.Text) > common.MaxLenBody:
		return errSpliceTooLong // Nice try, kid
	}

	for _, r := range req.Text {
		if r == 0 {
			return common.ErrContainsNull
		}
	}

	var (
		old = []rune(string(c.post.body))
		end []rune
	)

	// -1 has special meaning - slice off till end
	if req.Len == -1 {
		end = req.Text
		c.post.len = req.Start + len(req.Text)
	} else {
		end = append(req.Text, old[req.Start+req.Len:]...)
		c.post.len += -req.Len + len(req.Text)
	}
	res := spliceMessage{
		ID: c.post.id,
		spliceRequestString: spliceRequestString{
			spliceCoords: req.spliceCoords,
			Text:         string(req.Text),
		},
	}

	// If it goes over the max post length, trim the end
	exceeding := c.post.len - common.MaxLenBody
	if exceeding > 0 {
		end = end[:len(end)-exceeding]
		res.Len = -1
		res.Text = string(end)
		c.post.len = common.MaxLenBody
	}

	msg, err := common.EncodeMessage(common.MessageSplice, res)
	if err != nil {
		return err
	}

	// Need to prevent modifications to the original slice, as there might be
	// concurrent reads in the update feed.
	c.post.body = util.CloneBytes(c.post.body)

	byteStartPos := 0
	for _, r := range old[:req.Start] {
		byteStartPos += utf8.RuneLen(r)
	}

	c.post.body = append(c.post.body[:byteStartPos], string(end)...)

	// Count lines
	c.post.lines = 0
	for _, b := range c.post.body {
		if b == '\n' {
			c.post.lines++
		}
	}
	if c.post.lines > common.MaxLinesBody {
		return errTooManyLines
	}

	return c.updateBody(msg)
}

// Insert and image into an existing open post
func (c *Client) insertImage(data []byte) (err error) {
	has, err := c.hasPost()
	switch {
	case err != nil:
		return
	case !has:
		return errNoPostOpen
	case c.post.hasImage:
		return errHasImage
	}

	var req ImageRequest
	err = decodeMessage(data, &req)
	if err != nil {
		return
	}

	if config.GetBoardConfigs(c.post.board).TextOnly {
		return errTextOnly
	}

	img, err := getImage(req.Token, req.Name, req.Spoiler)
	if err != nil {
		return
	}
	c.post.hasImage = true
	c.post.isSpoilered = req.Spoiler

	err = db.InsertImage(c.post.id, c.post.op, *img)
	if err != nil {
		return
	}

	msg, err := common.EncodeMessage(common.MessageInsertImage, struct {
		ID uint64 `json:"id"`
		common.Image
	}{
		ID:    c.post.id,
		Image: *img,
	})
	if err != nil {
		return
	}
	c.feed.InsertImage(c.post.id, msg)

	return nil
}

// Spoiler an already inserted image in an unclosed post
func (c *Client) spoilerImage() error {
	has, err := c.hasPost()
	switch {
	case err != nil:
		return err
	case !has:
		return errNoPostOpen
	case !c.post.hasImage:
		return errors.New("post does not have an image")
	case c.post.isSpoilered:
		return errors.New("already spoilered")
	}

	return db.SpoilerImage(c.post.id)
}
