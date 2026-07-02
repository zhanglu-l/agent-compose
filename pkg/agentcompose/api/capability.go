package api

import (
	"strings"

	"agent-compose/pkg/agentcompose/domain"
	"agent-compose/pkg/capability"
	agentcomposev1 "agent-compose/proto/agentcompose/v1"
)

func CapabilityGatewayConfigToProto(settings domain.CapabilityGatewaySettings) *agentcomposev1.CapabilityGatewayConfig {
	return &agentcomposev1.CapabilityGatewayConfig{
		Addr:     settings.Addr,
		TokenSet: strings.TrimSpace(settings.Token) != "",
	}
}

func CapabilityCatalogToProto(item capability.Catalog) *agentcomposev1.GetCapabilityCatalogResponse {
	resp := &agentcomposev1.GetCapabilityCatalogResponse{
		CapsetId:    item.CapsetID,
		Name:        item.Name,
		Description: item.Description,
	}
	for _, method := range item.Methods {
		resp.Methods = append(resp.Methods, CapabilityMethodToProto(method))
	}
	return resp
}

func CapabilityMethodToProto(item capability.Method) *agentcomposev1.CapabilityMethod {
	resp := &agentcomposev1.CapabilityMethod{
		ServiceId:               item.ServiceID,
		InstanceId:              item.InstanceID,
		RuntimeMode:             item.RuntimeMode,
		MethodFullName:          item.MethodFullName,
		RequestMessageFullName:  item.RequestMessageFullName,
		ResponseMessageFullName: item.ResponseMessageFullName,
		BackendInstanceStatus:   item.BackendInstanceStatus,
	}
	for _, endpoint := range item.Endpoints {
		resp.Endpoints = append(resp.Endpoints, &agentcomposev1.CapabilityEndpoint{
			Protocol:     endpoint.Protocol,
			Endpoint:     endpoint.Endpoint,
			MethodPath:   endpoint.MethodPath,
			Metadata:     endpoint.Metadata,
			ToolName:     endpoint.ToolName,
			Procedure:    endpoint.Procedure,
			HttpMethod:   endpoint.HTTPMethod,
			ContentTypes: endpoint.ContentTypes,
		})
	}
	return resp
}
