// Package models holds the small request DTOs shared between internal/movies
// and internal/series — both controllers accept the exact same comment
// request shapes in Java (feather.movies.models.dtos.PostCommentRequest /
// UpdateCommentRequest, commonData.dtos.CinemaRecRequestDto), each with a
// single @NotBlank-validated field.
//
// Everything else this gateway proxies (full Movie/Series objects, watchlist
// items, subscriptions, ...) is forwarded as opaque JSON bytes rather than
// decoded into a parallel struct here — this gateway never reads or
// transforms those fields, so redefining their shape a second time would
// only be one more place for the two copies to drift apart. See
// internal/downstream for the byte-passthrough helper.
package models

import "errors"

// PostCommentRequest mirrors PostCommentRequest — deliberately just these
// two fields; author identity/createdAt come from the verified Clerk session
// and lumo-user-svc's own record instead (see internal/movies, internal/series).
type PostCommentRequest struct {
	CommentText    string `json:"commentText"`
	CinemaRecordID string `json:"cinemaRecordId"`
}

func (r PostCommentRequest) Validate() error {
	if isBlank(r.CommentText) {
		return errors.New("Comment text cannot be blank!")
	}
	if isBlank(r.CinemaRecordID) {
		return errors.New("Cinema Record id cannot be blank!")
	}
	return nil
}

// UpdateCommentRequest mirrors UpdateCommentRequest.
type UpdateCommentRequest struct {
	CommentText string `json:"commentText"`
}

func (r UpdateCommentRequest) Validate() error {
	if isBlank(r.CommentText) {
		return errors.New("Comment text cannot be blank!")
	}
	return nil
}

// CinemaRecRequestDto mirrors commonData.dtos.CinemaRecRequestDto.
type CinemaRecRequestDto struct {
	RecordName string `json:"recordName"`
}

func (r CinemaRecRequestDto) Validate() error {
	if isBlank(r.RecordName) {
		return errors.New("Record Name cannot be empty!")
	}
	return nil
}

func isBlank(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}
