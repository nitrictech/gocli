// Copyright Nitric Pty Ltd.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package collector

import (
	"context"
	"fmt"
	"sync"

	"github.com/samber/lo"

	apispb "github.com/nitrictech/nitric/core/pkg/proto/apis/v1"
	httppb "github.com/nitrictech/nitric/core/pkg/proto/http/v1"
	resourcespb "github.com/nitrictech/nitric/core/pkg/proto/resources/v1"
	schedulespb "github.com/nitrictech/nitric/core/pkg/proto/schedules/v1"
	storagepb "github.com/nitrictech/nitric/core/pkg/proto/storage/v1"
	topicspb "github.com/nitrictech/nitric/core/pkg/proto/topics/v1"
	websocketspb "github.com/nitrictech/nitric/core/pkg/proto/websockets/v1"
)

// ServiceRequirements - Cloud resource requirements for a Nitric Application Service
//
// Hosts all Nitric resource servers in a collection-only mode, where services can call into the servers to request resources they require for their operation.
type ServiceRequirements struct {
	serviceName string
	serviceType string

	resourceLock sync.Mutex

	routes        map[string][]*apispb.RegistrationRequest
	schedules     map[string]*schedulespb.RegistrationRequest
	subscriptions map[string][]*topicspb.RegistrationRequest
	websockets    map[string][]*websocketspb.RegistrationRequest
	listeners     map[string]*storagepb.RegistrationRequest

	proxy                 *httppb.HttpProxyRequest
	apis                  map[string]*resourcespb.ApiResource
	apiSecurityDefinition map[string]map[string]*resourcespb.ApiSecurityDefinitionResource
	buckets               map[string]*resourcespb.BucketResource
	collections           map[string]*resourcespb.CollectionResource
	topics                map[string]*resourcespb.TopicResource

	policies map[string]*resourcespb.PolicyResource
	secrets  map[string]*resourcespb.SecretResource

	errors []error
	topicspb.UnimplementedTopicsServer
	storagepb.UnimplementedStorageListenerServer
}

var (
	_ apispb.ApiServer                    = (*ServiceRequirements)(nil)
	_ schedulespb.SchedulesServer         = (*ServiceRequirements)(nil)
	_ topicspb.SubscriberServer           = (*ServiceRequirements)(nil)
	_ topicspb.TopicsServer               = (*ServiceRequirements)(nil)
	_ storagepb.StorageListenerServer     = (*ServiceRequirements)(nil)
	_ websocketspb.WebsocketHandlerServer = (*ServiceRequirements)(nil)
)

var _ resourcespb.ResourcesServer = (*ServiceRequirements)(nil)

func (s *ServiceRequirements) Details(context.Context, *resourcespb.ResourceDetailsRequest) (*resourcespb.ResourceDetailsResponse, error) {
	return &resourcespb.ResourceDetailsResponse{}, nil
}

func (s *ServiceRequirements) WorkerCount() int {
	return len(lo.Values(s.routes)) +
		len(s.listeners) +
		len(s.schedules) +
		len(lo.Values(s.subscriptions)) +
		len(lo.Values(s.websockets))
}

func (s *ServiceRequirements) Declare(ctx context.Context, req *resourcespb.ResourceDeclareRequest) (*resourcespb.ResourceDeclareResponse, error) {
	switch req.Resource.Type {
	case resourcespb.ResourceType_Bucket:
		// Add a bucket
		s.buckets[req.Resource.GetName()] = req.GetBucket()
	case resourcespb.ResourceType_Collection:
		// Add a collection
		s.collections[req.Resource.GetName()] = req.GetCollection()
	case resourcespb.ResourceType_Api:
		// Add an api
		s.apis[req.Resource.GetName()] = req.GetApi()
	case resourcespb.ResourceType_ApiSecurityDefinition:
		// Add an api security definition
		if s.apiSecurityDefinition[req.GetApiSecurityDefinition().GetApiName()] == nil {
			s.apiSecurityDefinition[req.GetApiSecurityDefinition().GetApiName()] = make(map[string]*resourcespb.ApiSecurityDefinitionResource)
		}

		s.apiSecurityDefinition[req.GetApiSecurityDefinition().GetApiName()][req.Resource.GetName()] = req.GetApiSecurityDefinition()
	case resourcespb.ResourceType_Secret:
		// Add a secret
		s.secrets[req.Resource.GetName()] = req.GetSecret()
	case resourcespb.ResourceType_Policy:
		// Services don't know their own name, so we need to add it here
		if req.GetPolicy().GetPrincipals() == nil {
			req.GetPolicy().Principals = []*resourcespb.Resource{{
				Name: s.serviceName,
				Type: resourcespb.ResourceType_Function,
			}}
		} else {
			for _, principal := range req.GetPolicy().GetPrincipals() {
				if principal.GetName() == "" && principal.GetType() == resourcespb.ResourceType_Function {
					principal.Name = s.serviceName
				}
			}
		}
		// Add a policy
		s.policies[req.Resource.GetName()] = req.GetPolicy()
	case resourcespb.ResourceType_Topic:
		// add a topic
		s.topics[req.Resource.GetName()] = req.GetTopic()
	}

	return &resourcespb.ResourceDeclareResponse{}, nil
}

func (s *ServiceRequirements) Proxy(ctx context.Context, req *httppb.HttpProxyRequest) (*httppb.HttpProxyResponse, error) {
	// capture a http proxy
	if len(s.routes) > 0 {
		s.errors = append(s.errors, fmt.Errorf("cannot register HTTP proxy, API routes have already been registered"))
	}

	if s.proxy != nil {
		s.errors = append(s.errors, fmt.Errorf("cannot register HTTP proxy, another proxy has already been registered"))
	}

	s.proxy = req

	return &httppb.HttpProxyResponse{}, nil
}

func (s *ServiceRequirements) Serve(stream apispb.Api_ServeServer) error {
	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	registrationRequest := msg.GetRegistrationRequest()

	if registrationRequest == nil {
		return fmt.Errorf("first message must be a registration request")
	}

	existingRoute, found := lo.Find(s.routes[registrationRequest.Api], func(item *apispb.RegistrationRequest) bool {
		return len(lo.Intersect(item.Methods, registrationRequest.Methods)) > 0 && item.Path == registrationRequest.Path
	})

	if found {
		s.errors = append(s.errors, fmt.Errorf("route already registered: %s %s", existingRoute.Api, existingRoute.Path))
	} else {
		s.routes[registrationRequest.Api] = append(s.routes[registrationRequest.Api], registrationRequest)
	}

	return stream.Send(&apispb.ServerMessage{
		Content: &apispb.ServerMessage_RegistrationResponse{
			RegistrationResponse: &apispb.RegistrationResponse{},
		},
	})
}

func (s *ServiceRequirements) Schedule(stream schedulespb.Schedules_ScheduleServer) error {
	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	registrationRequest := msg.GetRegistrationRequest()

	if registrationRequest == nil {
		return fmt.Errorf("first message must be a registration request")
	}

	_, found := s.schedules[registrationRequest.ScheduleName]
	if found {
		s.errors = append(s.errors, fmt.Errorf("schedule already registered: %s", registrationRequest.ScheduleName))
	}

	s.schedules[registrationRequest.ScheduleName] = registrationRequest

	return stream.Send(&schedulespb.ServerMessage{
		Content: &schedulespb.ServerMessage_RegistrationResponse{
			RegistrationResponse: &schedulespb.RegistrationResponse{},
		},
	})
}

func (s *ServiceRequirements) Subscribe(stream topicspb.Subscriber_SubscribeServer) error {
	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	registrationRequest := msg.GetRegistrationRequest()

	if registrationRequest == nil {
		return fmt.Errorf("first message must be a registration request")
	}

	s.subscriptions[registrationRequest.TopicName] = append(s.subscriptions[registrationRequest.TopicName], registrationRequest)

	return stream.Send(&topicspb.ServerMessage{
		Content: &topicspb.ServerMessage_RegistrationResponse{
			RegistrationResponse: &topicspb.RegistrationResponse{},
		},
	})
}

func (s *ServiceRequirements) Listen(stream storagepb.StorageListener_ListenServer) error {
	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	registrationRequest := msg.GetRegistrationRequest()

	if registrationRequest == nil {
		return fmt.Errorf("first ")
	}

	_, found := s.buckets[registrationRequest.BucketName]

	if found {
		s.errors = append(s.errors, fmt.Errorf("bucket already registered: %s", registrationRequest.BucketName))
	} else {
		s.listeners[registrationRequest.BucketName] = registrationRequest
	}

	return stream.Send(&storagepb.ServerMessage{
		Content: &storagepb.ServerMessage_RegistrationResponse{
			RegistrationResponse: &storagepb.RegistrationResponse{},
		},
	})
}

func (s *ServiceRequirements) HandleEvents(stream websocketspb.WebsocketHandler_HandleEventsServer) error {
	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	registrationRequest := msg.GetRegistrationRequest()

	if registrationRequest == nil {
		return fmt.Errorf("first message must be a registration request")
	}

	existingSocketHandler, found := lo.Find(s.websockets[registrationRequest.SocketName], func(item *websocketspb.RegistrationRequest) bool {
		return item.EventType == registrationRequest.EventType
	})

	if found {
		s.errors = append(s.errors, fmt.Errorf("websocket handler already registered: %s %s", existingSocketHandler.SocketName, existingSocketHandler.EventType))
	} else {
		s.websockets[registrationRequest.SocketName] = append(s.websockets[registrationRequest.SocketName], registrationRequest)
	}

	return stream.Send(&websocketspb.ServerMessage{
		Content: &websocketspb.ServerMessage_RegistrationResponse{
			RegistrationResponse: &websocketspb.RegistrationResponse{},
		},
	})
}

func NewServiceRequirements(serviceName string, serviceType string) *ServiceRequirements {
	if serviceType == "" {
		serviceType = "default"
	}

	return &ServiceRequirements{
		serviceName:           serviceName,
		serviceType:           serviceType,
		resourceLock:          sync.Mutex{},
		routes:                make(map[string][]*apispb.RegistrationRequest),
		schedules:             make(map[string]*schedulespb.RegistrationRequest),
		subscriptions:         make(map[string][]*topicspb.RegistrationRequest),
		websockets:            make(map[string][]*websocketspb.RegistrationRequest),
		buckets:               make(map[string]*resourcespb.BucketResource),
		collections:           make(map[string]*resourcespb.CollectionResource),
		topics:                make(map[string]*resourcespb.TopicResource),
		policies:              make(map[string]*resourcespb.PolicyResource),
		secrets:               make(map[string]*resourcespb.SecretResource),
		listeners:             make(map[string]*storagepb.RegistrationRequest),
		apis:                  make(map[string]*resourcespb.ApiResource),
		apiSecurityDefinition: make(map[string]map[string]*resourcespb.ApiSecurityDefinitionResource),
		errors:                []error{},
	}
}