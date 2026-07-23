package agentcomposev2

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestTimePointFieldsUseProtobufTimestamp(t *testing.T) {
	t.Parallel()
	assertMessageTimePointsUseTimestamp(t, File_agentcompose_v2_agentcompose_proto.Messages())
}

func assertMessageTimePointsUseTimestamp(t *testing.T, messages protoreflect.MessageDescriptors) {
	t.Helper()
	for i := 0; i < messages.Len(); i++ {
		message := messages.Get(i)
		for j := 0; j < message.Fields().Len(); j++ {
			field := message.Fields().Get(j)
			name := string(field.Name())
			if name != "at" && !strings.HasSuffix(name, "_at") && !strings.HasSuffix(name, "_time") {
				continue
			}
			if field.Message() == nil || field.Message().FullName() != "google.protobuf.Timestamp" {
				t.Errorf("%s must use google.protobuf.Timestamp, got %s", field.FullName(), field.Kind())
			}
		}
		assertMessageTimePointsUseTimestamp(t, message.Messages())
	}
}
