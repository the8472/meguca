package db

import (
	"meguca/common"
)

// Writes new backlinks to other posts
func insertBackinks(id, op uint64, links [][2]uint64) (err error) {
	// Deduplicate
	dedupped := make(map[[2]uint64]struct{}, len(links))
	for _, l := range links {
		dedupped[l] = struct{}{}
	}

	// Most often this loop will iterate only once, so no need to think heavily
	// on optimizations
	for l := range dedupped {
		var msg []byte
		msg, err = common.EncodeMessage(
			common.MessageBacklink,
			[3]uint64{l[0], id, op},
		)
		if err != nil {
			return
		}

		err = execPrepared("insert_backlink", l[0], linkRow{{id, op}})
		if err != nil {
			return
		}

		// nil during tests
		if !IsTest {
			common.SendTo(op, msg)
		}
	}

	return
}

// ClosePost closes an open post and commits any links, backlinks and hash
// commands
func ClosePost(id, op uint64, body string, links [][2]uint64, com []common.Command) (
	err error,
) {
	msg, err := common.EncodeMessage(common.MessageClosePost, struct {
		ID       uint64           `json:"id"`
		Links    [][2]uint64      `json:"links,omitempty"`
		Commands []common.Command `json:"commands,omitempty"`
	}{
		ID:       id,
		Links:    links,
		Commands: com,
	})
	if err != nil {
		return err
	}

	err = execPrepared(
		"close_post",
		id, op, body, linkRow(links), commandRow(com),
	)
	if err != nil {
		return
	}

	if links != nil {
		err = insertBackinks(id, op, links)
		if err != nil {
			return
		}
	}

	if !IsTest {
		common.ClosePost(id, op, msg)
	}
	return deleteOpenPostBody(id)
}
