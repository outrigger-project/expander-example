/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package expander

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"log"
	"net"
	"sort"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/autoscaler/cluster-autoscaler/expander/grpcplugin/protos"
)

// This code is meant to be used as starter code, deployed as a separate app, not in Cluster Autoscaler.
// This serves as the gRPC Expander Server counterpart to the client which lives in this repo
// main.go of said application should simply pass in paths to (optional)cert, (optional)private key, and port, and call Serve to start listening
// copy the protos/expander.pb.go to your other application's repo, so it has access to the protobuf definitions

// Serve should be called by the main() function in main.go of the Expander Server repo to start serving
func Serve(certPath string, keyPath string, port uint) {

	var grpcServer *grpc.Server

	// If credentials are passed in, use them
	if certPath != "" && keyPath != "" {
		log.Printf("Using certFile: %v and keyFile: %v", certPath, keyPath)
		tlsCredentials, err := credentials.NewServerTLSFromFile(certPath, keyPath)
		if err != nil {
			log.Fatal("cannot load TLS credentials: ", err)
		}
		grpcServer = grpc.NewServer(grpc.Creds(tlsCredentials))
	} else {
		grpcServer = grpc.NewServer()
	}

	netListener := getNetListener(port)

	expanderServerImpl := NewExpanderServerImpl()

	protos.RegisterExpanderServer(grpcServer, expanderServerImpl)

	// start the server
	log.Println("Starting server on port ", port)
	if err := grpcServer.Serve(netListener); err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}

func getNetListener(port uint) net.Listener {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	return lis
}

// ExpanderServerImpl is an implementation of Expander Server from proto definition
type ExpanderServerImpl struct{}

// NewExpanderServerImpl is this Expander's implementation of the server
func NewExpanderServerImpl() *ExpanderServerImpl {
	return &ExpanderServerImpl{}
}

// BestOptions method filters out the best options of all options passed from the gRPC Client in CA, according to the defined strategy.
func (ServerImpl *ExpanderServerImpl) BestOptions(ctx context.Context, req *protos.BestOptionsRequest) (*protos.BestOptionsResponse, error) {
	opts := req.GetOptions()
	nodeMap := req.GetNodeMap()
	log.Printf("Received BestOption Request with %v options", len(opts))

	// Sort by preference affinity
	const replicasDiscount = 0.35
	sort.Slice(opts, func(i, j int) bool {
		return calculateScore(opts[i], nodeMap[opts[i].NodeGroupId], replicasDiscount) >
			calculateScore(opts[j], nodeMap[opts[j].NodeGroupId], replicasDiscount)
	})

	// Return just one option for now
	return &protos.BestOptionsResponse{
		Options: []*protos.Option{opts[0]},
	}, nil
}

// calculateScore calculates the score of a node based on the affinity rules defined in the pod spec and the given node.
func calculateScore(opt *protos.Option, node *v1.Node, replicasDiscount float64) float64 {
	var totalScore float64
	for _, pod := range opt.Pod {
		var podScore int64
		for _, term := range pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			for _, matchExpr := range term.Preference.MatchExpressions {
				selector := labels.NewSelector()
				req, err := labels.NewRequirement(matchExpr.Key, selection.Operator(matchExpr.Operator), matchExpr.Values)
				if err != nil {
					log.Printf("Error creating label requirement: %v", err)
					continue
				}
				selector = selector.Add(*req)
				if selector.Matches(labels.Set(node.Labels)) {
					podScore += int64(term.Weight)
				}
			}
		}
		totalScore += float64(podScore) / (1 + (float64(opt.NodeCount-1) * replicasDiscount))
	}
	return totalScore
}
