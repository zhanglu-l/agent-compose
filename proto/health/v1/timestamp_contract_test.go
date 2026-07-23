package healthv1

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestHealthTimePointsUseProtobufTimestamp(t *testing.T) {
	t.Parallel()
	fields := File_health_v1_health_proto.Messages().ByName("HealthStatusResponse").Fields()
	for _, name := range []string{"current_time", "started_at"} {
		field := fields.ByName(protoreflect.Name(name))
		if field.Message() == nil || field.Message().FullName() != "google.protobuf.Timestamp" {
			t.Errorf("%s must use google.protobuf.Timestamp, got %s", field.FullName(), field.Kind())
		}
	}
}
