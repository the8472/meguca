package websockets

import (
	"bytes"
	"testing"
	"time"

	"github.com/bakape/meguca/common"
	"github.com/bakape/meguca/config"
	"github.com/bakape/meguca/db"
	. "github.com/bakape/meguca/test"
	r "github.com/dancannon/gorethink"
)

var (
	// JPEG sample image standard struct
	stdJPEG = common.ImageCommon{
		SHA1:     "012a2f912c9ee93ceb0ccb8684a29ec571990a94",
		FileType: 0,
		Dims:     [4]uint16{1, 1, 1, 1},
		MD5:      "YOQQklgfezKbBXuEAsqopw",
		Size:     300792,
	}

	sampleImagelessThreadCreationRequest = ThreadCreationRequest{
		ReplyCreationRequest: ReplyCreationRequest{
			Name:     "name",
			Password: "123",
		},
		Subject: "subject",
		Board:   "a",
	}
)

func TestInsertThread(t *testing.T) {
	assertTableClear(t, "main", "posts", "threads", "images")
	populateMainTable(t)

	conf := [...]config.BoardConfigs{
		{
			ID: "c",
		},
		{
			ID: "r",
			BoardPublic: config.BoardPublic{
				PostParseConfigs: config.PostParseConfigs{
					ReadOnly: true,
				},
			},
		},
		{
			ID: "a",
			BoardPublic: config.BoardPublic{
				PostParseConfigs: config.PostParseConfigs{
					TextOnly: true,
				},
			},
		},
	}
	for _, c := range conf {
		_, err := config.SetBoardConfigs(c)
		if err != nil {
			t.Fatal(err)
		}
	}

	cases := [...]struct {
		name, board string
		err         error
	}{
		{"invalid board", "all", errInvalidBoard},
		{"invalid board", "x", errInvalidBoard},
		{"read-only board", "r", errReadOnly},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			req := ThreadCreationRequest{
				Board: c.board,
			}
			err := insertThread(marshalJSON(t, req), new(Client))
			if err != c.err {
				UnexpectedError(t, err)
			}
		})
	}

	t.Run("with image", testCreateThread)
	t.Run("text only board", testCreateThreadTextOnly)
}

func testCreateThread(t *testing.T) {
	assertInsert(t, "images", stdJPEG)
	_, token, err := db.NewImageToken(stdJPEG.SHA1)
	if err != nil {
		t.Fatal(err)
	}

	sv := newWSServer(t)
	defer sv.Close()
	cl, wcl := sv.NewClient()
	Clients.add(cl, SyncID{
		OP:    0,
		Board: "all",
	})
	defer Clients.Clear()
	cl.ip = "::1"

	stdThread := common.DatabaseThread{
		ID:       6,
		Subject:  "subject",
		Board:    "c",
		ImageCtr: 1,
	}
	stdPost := common.DatabasePost{
		IP:  "::1",
		Log: [][]byte{},
		StandalonePost: common.StandalonePost{
			Post: common.Post{
				Editing: true,
				ID:      6,
				Name:    "name",
				Image: &common.Image{
					Spoiler:     true,
					ImageCommon: stdJPEG,
					Name:        "foo",
				},
			},
			Board: "c",
			OP:    6,
		},
	}

	req := ThreadCreationRequest{
		ReplyCreationRequest: ReplyCreationRequest{
			Name:     "name",
			Password: "123",
			Image: ImageRequest{
				Name:    "foo.jpeg",
				Token:   token,
				Spoiler: true,
			},
		},
		Subject: "subject",
		Board:   "c",
	}
	data := marshalJSON(t, req)
	if err := insertThread(data, cl); err != nil {
		t.Fatal(err)
	}
	assertMessage(t, wcl, `326`)
	assertIP(t, 6, "::1")

	var (
		thread common.DatabaseThread
		post   common.DatabasePost
	)
	if err := db.One(db.FindThread(6), &thread); err != nil {
		t.Fatal(err)
	}
	if err := db.One(db.FindPost(6), &post); err != nil {
		t.Fatal(err)
	}

	// Pointers have to be dereferenced to be asserted
	AssertDeepEquals(t, *post.Image, *stdPost.Image)

	// Normalize timestamps and pointer fields
	then := thread.ReplyTime
	stdThread.ReplyTime = then
	stdPost.Time = then
	stdPost.LastUpdated = then
	stdPost.Password = post.Password
	stdPost.Image = post.Image

	AssertDeepEquals(t, thread, stdThread)
	AssertDeepEquals(t, post, stdPost)
	AssertDeepEquals(t, cl.openPost, openPost{
		id:       6,
		op:       6,
		board:    "c",
		time:     then,
		hasImage: true,
	})
}

func testCreateThreadTextOnly(t *testing.T) {
	sv := newWSServer(t)
	defer sv.Close()
	cl, wcl := sv.NewClient()
	data := marshalJSON(t, sampleImagelessThreadCreationRequest)
	if err := insertThread(data, cl); err != nil {
		t.Fatal(err)
	}
	assertMessage(t, wcl, `327`)
	if cl.openPost.hasImage {
		t.Error("image inserted")
	}

	var noImage bool
	q := db.FindPost(7).HasFields("image").Not()
	if err := db.One(q, &noImage); err != nil {
		t.Fatal(err)
	}
	if !noImage {
		t.Error("image written to database")
	}
}

func populateMainTable(t testing.TB) {
	assertInsert(t, "main", map[string]interface{}{
		"id":      "info",
		"postCtr": 5,
	})
}

func setBoardConfigs(t testing.TB, textOnly bool) {
	config.ClearBoards()
	_, err := config.SetBoardConfigs(config.BoardConfigs{
		ID: "a",
		BoardPublic: config.BoardPublic{
			PostParseConfigs: config.PostParseConfigs{
				TextOnly: textOnly,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertIP(t *testing.T, id uint64, ip string) {
	q := db.FindPost(id).Field("ip")
	var res string
	if err := db.One(q, &res); err != nil {
		t.Fatal(err)
	}
	if res != ip {
		t.Errorf("unexpected ip: %s : %s", ip, res)
	}
}

func TestGetInvalidImage(t *testing.T) {
	assertTableClear(t, "images")

	const (
		name  = "foo.jpeg"
		token = "dasdasd-ad--dsad-ads-d-ad-"
	)

	cases := [...]struct {
		testName, token, name string
		err                   error
	}{
		{"token too long", GenString(128), name, errInvalidImageToken},
		{"image name too long", token, GenString(201), errImageNameTooLong},
		{"no token in DB", token, name, errInvalidImageToken},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.testName, func(t *testing.T) {
			t.Parallel()
			if _, err := getImage(c.token, c.name, false); err != c.err {
				UnexpectedError(t, err)
			}
		})
	}
}

func TestClosePreviousPostOnCreation(t *testing.T) {
	assertTableClear(t, "main", "threads", "posts")
	assertInsert(t, "posts", samplePost)
	populateMainTable(t)
	setBoardConfigs(t, true)

	sv := newWSServer(t)
	defer sv.Close()
	cl, wcl := sv.NewClient()
	cl.openPost = openPost{
		id:         2,
		op:         1,
		bodyLength: 3,
		board:      "a",
		time:       time.Now().Unix(),
		Buffer:     *bytes.NewBuffer([]byte("abc")),
	}
	data := marshalJSON(t, sampleImagelessThreadCreationRequest)

	if err := insertThread(data, cl); err != nil {
		t.Fatal(err)
	}

	assertMessage(t, wcl, `326`)
	assertRepLog(t, 2, append(strDummyLog, "062"))
	assertPostClosed(t, 2)
}

func TestPostCreationValidations(t *testing.T) {
	setBoardConfigs(t, false)

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	Clients.add(cl, SyncID{1, "a"})

	cases := [...]struct {
		testName, token, name string
	}{
		{"no token", "", "abc"},
		{"no image name", "abc", ""},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.testName, func(t *testing.T) {
			t.Parallel()

			req := ReplyCreationRequest{
				Image: ImageRequest{
					Name:  c.name,
					Token: c.token,
				},
			}
			err := insertPost(marshalJSON(t, req), cl)
			if err != errNoTextOrImage {
				UnexpectedError(t, err)
			}
		})
	}
}

func TestPostCreationOnLockedThread(t *testing.T) {
	assertTableClear(t, "threads")
	assertInsert(t, "threads", map[string]interface{}{
		"id":      1,
		"board":   "a",
		"postCtr": 0,
		"locked":  true,
	})
	setBoardConfigs(t, true)

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	Clients.add(cl, SyncID{1, "a"})
	defer Clients.Clear()

	req := ReplyCreationRequest{
		Body: "a",
	}
	if err := insertPost(marshalJSON(t, req), cl); err != errThreadIsLocked {
		UnexpectedError(t, err)
	}
}

func TestPostCreation(t *testing.T) {
	prepareForPostCreation(t)
	setBoardConfigs(t, false)
	assertInsert(t, "images", stdJPEG)
	_, token, err := db.NewImageToken(stdJPEG.SHA1)
	if err != nil {
		t.Fatal(err)
	}

	sv := newWSServer(t)
	defer sv.Close()
	cl, wcl := sv.NewClient()
	Clients.add(cl, SyncID{1, "a"})
	defer Clients.Clear()
	cl.ip = "::1"

	req := ReplyCreationRequest{
		Body:     "Δ",
		Password: "123",
		Image: ImageRequest{
			Name:    "foo.jpeg",
			Token:   token,
			Spoiler: true,
		},
	}

	if err := insertPost(marshalJSON(t, req), cl); err != nil {
		t.Fatal(err)
	}

	assertMessage(t, wcl, encodeMessageType(MessagePostID)+"6")

	// Get the time value from the DB and normalize against it
	var then int64
	if err := db.One(db.FindPost(6).Field("time"), &then); err != nil {
		t.Fatal(err)
	}

	stdPost := common.DatabasePost{
		StandalonePost: common.StandalonePost{
			Post: common.Post{
				Editing: true,
				ID:      6,
				Time:    then,
				Body:    "Δ",
				Image: &common.Image{
					Name:        "foo",
					Spoiler:     true,
					ImageCommon: stdJPEG,
				},
			},
			OP:    1,
			Board: "a",
		},
		IP:          "::1",
		LastUpdated: then,
	}

	var post common.DatabasePost
	if err := db.One(db.FindPost(6), &post); err != nil {
		t.Fatal(err)
	}
	AssertDeepEquals(t, *post.Image, *stdPost.Image)
	stdPost.Image = post.Image
	stdPost.Password = post.Password
	stdPost.Log = post.Log
	AssertDeepEquals(t, post, stdPost)

	assertIP(t, 6, "::1")

	// Assert thread was bumped
	type threadAttrs struct {
		PostCtr   int
		ImageCtr  int
		ReplyTime int64
	}

	var attrs threadAttrs
	q := db.FindThread(1).Pluck("postCtr", "imageCtr", "replyTime")
	if err := db.One(q, &attrs); err != nil {
		t.Fatal(err)
	}
	stdAttrs := threadAttrs{
		PostCtr:   1,
		ImageCtr:  2,
		ReplyTime: then,
	}
	if attrs != stdAttrs {
		LogUnexpected(t, stdAttrs, attrs)
	}

	AssertDeepEquals(t, cl.openPost, openPost{
		id:         6,
		op:         1,
		time:       then,
		board:      "a",
		bodyLength: 1,
		Buffer:     *bytes.NewBuffer([]byte("Δ")),
		hasImage:   true,
	})
}

func prepareForPostCreation(t testing.TB) {
	now := time.Now().Unix()
	assertTableClear(t, "main", "threads", "posts")
	assertInsert(t, "threads", common.DatabaseThread{
		ID:        1,
		Board:     "a",
		PostCtr:   0,
		ImageCtr:  1,
		ReplyTime: now,
	})
	populateMainTable(t)
}

func TestTextOnlyPostCreation(t *testing.T) {
	prepareForPostCreation(t)
	setBoardConfigs(t, true)

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	Clients.add(cl, SyncID{1, "a"})
	defer Clients.Clear()

	req := ReplyCreationRequest{
		Body:     "a",
		Password: "123",
	}

	if err := insertPost(marshalJSON(t, req), cl); err != nil {
		t.Fatal(err)
	}

	// Assert image counter did not change
	assertImageCounter(t, 1, 1)

	// Assert no image in post
	var hasImage bool
	q := db.FindPost(6).HasFields("image")
	if err := db.One(q, &hasImage); err != nil {
		t.Fatal(err)
	}
	if hasImage {
		t.Error("DB post has image")
	}

	if cl.openPost.hasImage {
		t.Error("openPost has image")
	}
}

func BenchmarkPostCreation(b *testing.B) {
	prepareForPostCreation(b)
	setBoardConfigs(b, true)

	sv := newWSServer(b)
	defer sv.Close()
	cl, _ := sv.NewClient()
	Clients.add(cl, SyncID{1, "a"})
	defer Clients.Clear()

	req := ReplyCreationRequest{
		Body:     "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
		Password: "123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := insertPost(marshalJSON(b, req), cl); err != nil {
			b.Fatal(err)
		}
		if err := closePost(nil, cl); err != nil {
			b.Fatal(err)
		}
	}
}

func TestPostCreationForcedAnon(t *testing.T) {
	prepareForPostCreation(t)
	config.ClearBoards()
	_, err := config.SetBoardConfigs(config.BoardConfigs{
		ID: "a",
		BoardPublic: config.BoardPublic{
			PostParseConfigs: config.PostParseConfigs{
				ForcedAnon: true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	sv := newWSServer(t)
	defer sv.Close()
	cl, _ := sv.NewClient()
	Clients.add(cl, SyncID{1, "a"})
	defer Clients.Clear()

	req := ReplyCreationRequest{
		Body:     "a",
		Password: "123",
	}

	if err := insertPost(marshalJSON(t, req), cl); err != nil {
		t.Fatal(err)
	}

	// Assert no name or trip in post
	var isAnon bool
	post := db.FindPost(6)
	q := r.And(
		post.HasFields("name").Not(),
		post.HasFields("trip").Not(),
	)
	if err := db.One(q, &isAnon); err != nil {
		t.Fatal(err)
	}
	if !isAnon {
		t.Fatal("not anonymous")
	}
}

func assertImageCounter(t *testing.T, id uint64, ctr int) {
	var res int
	q := db.FindThread(id).Field("imageCtr")
	if err := db.One(q, &res); err != nil {
		t.Fatal(err)
	}
	if res != ctr {
		t.Errorf("unexpected thread image counter: %d : %d", ctr, res)
	}
}
