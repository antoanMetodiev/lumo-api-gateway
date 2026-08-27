package models

import "testing"

func TestPostCommentRequestValidate(t *testing.T) {
	cases := []struct {
		name    string
		req     PostCommentRequest
		wantErr bool
	}{
		{"valid", PostCommentRequest{CommentText: "nice movie", CinemaRecordID: "rec_1"}, false},
		{"blank comment text", PostCommentRequest{CommentText: "", CinemaRecordID: "rec_1"}, true},
		{"whitespace-only comment text", PostCommentRequest{CommentText: "   \t\n", CinemaRecordID: "rec_1"}, true},
		{"blank cinema record id", PostCommentRequest{CommentText: "nice movie", CinemaRecordID: ""}, true},
		{"both blank", PostCommentRequest{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestPostCommentRequestValidateChecksCommentTextFirst mirrors the code's
// checked order: a request with both fields blank should surface the
// comment-text error, not the cinema-record-id one.
func TestPostCommentRequestValidateChecksCommentTextFirst(t *testing.T) {
	err := PostCommentRequest{}.Validate()
	if err == nil {
		t.Fatal("expected an error")
	}
	if err.Error() != "Comment text cannot be blank!" {
		t.Errorf("error = %q, want the comment-text message to win when both are blank", err.Error())
	}
}

func TestUpdateCommentRequestValidate(t *testing.T) {
	if err := (UpdateCommentRequest{CommentText: "updated"}).Validate(); err != nil {
		t.Errorf("unexpected error for a non-blank comment: %v", err)
	}
	if err := (UpdateCommentRequest{CommentText: ""}).Validate(); err == nil {
		t.Error("expected an error for a blank comment text")
	}
	if err := (UpdateCommentRequest{CommentText: "  "}).Validate(); err == nil {
		t.Error("expected an error for a whitespace-only comment text")
	}
}

func TestCinemaRecRequestDtoValidate(t *testing.T) {
	if err := (CinemaRecRequestDto{RecordName: "The Matrix"}).Validate(); err != nil {
		t.Errorf("unexpected error for a non-blank record name: %v", err)
	}
	if err := (CinemaRecRequestDto{RecordName: ""}).Validate(); err == nil {
		t.Error("expected an error for a blank record name")
	}
	if err := (CinemaRecRequestDto{RecordName: "\t \n"}).Validate(); err == nil {
		t.Error("expected an error for a whitespace-only record name")
	}
}
