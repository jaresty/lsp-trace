package regexlocator

import (
	"crypto/sha256"
	"fmt"
	"regexp"

	"lsp-trace/internal/sourceposition"
)

type Code string

const (
	CodeInvalidRequest  Code = "INVALID_REQUEST"
	CodeInvalidRegex    Code = "INVALID_REGEX"
	CodeEmptyMatch      Code = "EMPTY_MATCH"
	CodeDigestMismatch  Code = "DOCUMENT_DIGEST_MISMATCH"
	CodeResourceLimit   Code = "RESOURCE_LIMIT"
	CodeMatchAbsent     Code = "MATCH_INDEX_OUT_OF_RANGE"
	CodeCaptureAbsent   Code = "CAPTURE_GROUP_ABSENT"
	CodePositionInvalid Code = "POSITION_INVALID"
)

type Limits struct {
	MaxDocumentBytes int
	MaxMatches       int
	MaxPatternBytes  int
	MaxWork          int
}

type Request struct {
	Document       []byte
	Pattern        string
	MatchIndex     int
	CaptureGroup   int
	ExpectedDigest string
	Encoding       string
	Limits         Limits
}

type Result struct {
	Position   sourceposition.Position
	ByteOffset int
	MatchCount int
}

type Error struct {
	Code  Code
	Cause error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("regex locator: %s: %v", e.Code, e.Cause)
	}
	return fmt.Sprintf("regex locator: %s", e.Code)
}

func (e *Error) Unwrap() error { return e.Cause }

func fail(code Code, cause error) (Result, error) {
	return Result{}, &Error{Code: code, Cause: cause}
}

func Resolve(request Request) (Result, error) {
	limits := request.Limits
	if request.Pattern == "" || request.MatchIndex < 0 || request.CaptureGroup < 0 ||
		limits.MaxDocumentBytes <= 0 || limits.MaxMatches <= 0 || limits.MaxPatternBytes <= 0 || limits.MaxWork <= 0 {
		return fail(CodeInvalidRequest, nil)
	}
	if len(request.Document) > limits.MaxDocumentBytes || len(request.Pattern) > limits.MaxPatternBytes {
		return fail(CodeResourceLimit, nil)
	}
	work := len(request.Document) + len(request.Pattern)
	if work > limits.MaxWork {
		return fail(CodeResourceLimit, nil)
	}
	if request.ExpectedDigest != "" {
		digest := fmt.Sprintf("sha256:%x", sha256.Sum256(request.Document))
		if digest != request.ExpectedDigest {
			return fail(CodeDigestMismatch, nil)
		}
	}
	re, err := regexp.Compile(request.Pattern)
	if err != nil {
		return fail(CodeInvalidRegex, err)
	}
	if empty := re.FindStringIndex(""); empty != nil && empty[0] == empty[1] {
		return fail(CodeEmptyMatch, nil)
	}
	if request.CaptureGroup > re.NumSubexp() {
		return fail(CodeCaptureAbsent, nil)
	}
	matches := re.FindAllSubmatchIndex(request.Document, limits.MaxMatches+1)
	if len(matches) > limits.MaxMatches {
		return fail(CodeResourceLimit, nil)
	}
	if work+len(matches) > limits.MaxWork {
		return fail(CodeResourceLimit, nil)
	}
	if request.MatchIndex >= len(matches) {
		return fail(CodeMatchAbsent, nil)
	}
	match := matches[request.MatchIndex]
	pair := request.CaptureGroup * 2
	if pair+1 >= len(match) || match[pair] < 0 || match[pair+1] < 0 {
		return fail(CodeCaptureAbsent, nil)
	}
	offset := match[pair]
	position, err := sourceposition.PositionAtOffset(request.Document, request.Encoding, offset)
	if err != nil {
		return fail(CodePositionInvalid, err)
	}
	return Result{Position: position, ByteOffset: offset, MatchCount: len(matches)}, nil
}
