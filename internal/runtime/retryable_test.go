package runtime

import "testing"

func TestIsRetryableConvergeFailure(t *testing.T) {
	err := &ConvergeError{Err: testErr("failed"), Output: "ERROR: connection refused while contacting upstream"}
	if !IsRetryableConvergeFailure(err, DefaultRetryableExceptions) {
		t.Fatal("expected retryable failure")
	}
}

func TestIsRetryableConvergeFailureFalseForNonMatchingOutput(t *testing.T) {
	err := &ConvergeError{Err: testErr("failed"), Output: "syntax error in recipe"}
	if IsRetryableConvergeFailure(err, DefaultRetryableExceptions) {
		t.Fatal("expected non-retryable failure")
	}
}

type testErr string

func (e testErr) Error() string { return string(e) }
