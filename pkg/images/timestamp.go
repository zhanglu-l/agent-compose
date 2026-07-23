package images

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func timeToProto(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func unixSecondsToProto(value int64) *timestamppb.Timestamp {
	if value <= 0 {
		return nil
	}
	return timestamppb.New(time.Unix(value, 0))
}

func rfc3339ToProto(value string) *timestamppb.Timestamp {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return timestamppb.New(parsed)
}
