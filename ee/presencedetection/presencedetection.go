package presencedetection

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	DetectionFailedDurationValue = -1 * time.Second
	DetectionTimeout             = 1 * time.Minute
)

type PresenceDetector struct {
	lastDetection time.Time
	mutex         sync.Mutex
	// detector is an interface to allow for mocking in tests
	detector detectorIface
}

// just exists for testing purposes
type detectorIface interface {
	Detect(reason string) (bool, error)
}

type detector struct{}

func (d *detector) Detect(reason string) (bool, error) {
	return Detect(reason)
}

// DetectPresence checks if the user is present by detecting the presence of a user.
// It returns the duration since the last detection.
func (pd *PresenceDetector) DetectPresence(reason string, detectionInterval time.Duration) (time.Duration, error) {
	// using try lock here because we don't don't want presence detections to queue up,
	// in the event that the users presses cancel, if the request were queued up, it would
	// request the presence detection again
	if !pd.mutex.TryLock() {
		return DetectionFailedDurationValue, errors.New("detection already in progress")
	}
	defer pd.mutex.Unlock()

	if pd.detector == nil {
		pd.detector = &detector{}
	}

	// Check if the last detection was within the detection interval
	if (pd.lastDetection != time.Time{}) && time.Since(pd.lastDetection) < detectionInterval {
		return time.Since(pd.lastDetection), nil
	}

	success, err := pd.detector.Detect(reason)
	if err != nil {
		// if we got an error, we behave as if there have been no successful detections in the past
		return DetectionFailedDurationValue, fmt.Errorf("detecting presence: %w", err)
	}

	if success {
		pd.lastDetection = time.Now().UTC()
		return 0, nil
	}

	// if we got here it means we failed without an error
	// this "should" never happen, but here for completeness
	return DetectionFailedDurationValue, fmt.Errorf("detection failed without OS error")
}
