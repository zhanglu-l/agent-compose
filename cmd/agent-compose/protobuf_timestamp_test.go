package main

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func mustProtoTimestamp(value string) *timestamppb.Timestamp {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return timestamppb.New(parsed)
}
