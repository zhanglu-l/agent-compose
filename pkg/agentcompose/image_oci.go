package agentcompose

import (
	"time"

	"agent-compose/pkg/agentcompose/images"
	"agent-compose/pkg/imagecache"
	agentcomposev2 "agent-compose/proto/agentcompose/v2"
)

func ociMetadataToProtoImage(image imagecache.ImageMetadata, inspectedAt string) *agentcomposev2.Image {
	return images.OCIMetadataToProtoImage(image, inspectedAt)
}

func cleanOCIRefs(refs []string) []string {
	return images.CleanOCIRefs(refs)
}

func timeString(value time.Time) string {
	return images.TimeString(value)
}
