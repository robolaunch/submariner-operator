/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

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

package network

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/submariner-io/submariner/pkg/cni"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	registerNetworkPluginDiscoveryFunction(discoverOpenShift4Network)
}

//nolint:nilnil // Intentional as the purpose is to discover.
func discoverOpenShift4Network(client controllerClient.Client) (*ClusterNetwork, error) {
	network := &unstructured.Unstructured{}
	network.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "config.openshift.io",
		Kind:    "Network",
		Version: "v1",
	})

	err := client.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, network)
	if err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil, nil
		}

		return nil, errors.WithMessage(err, "error obtaining the default 'cluster' OpenShift4 Network config resource")
	}

	return parseOS4Network(network)
}

func parseOS4Network(cr *unstructured.Unstructured) (*ClusterNetwork, error) {
	result := &ClusterNetwork{}

	clusterNetworks, found, err := unstructured.NestedSlice(cr.Object, "spec", "clusterNetwork")
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving spec.clusterNetwork field")
	} else if !found {
		return nil, fmt.Errorf("field .spec.clusterNetwork expected, but not found in Network resource: %v", cr.Object)
	}

	for _, clusterNetwork := range clusterNetworks {
		clusterNetworkMap, _ := clusterNetwork.(map[string]interface{})
		cidr, found, err := unstructured.NestedString(clusterNetworkMap, "cidr")

		if err != nil {
			return nil, errors.Wrap(err, "error retrieving cidr field")
		} else if !found {
			return nil, fmt.Errorf("field cidr expected, but not found in clusterNetwork: %v", clusterNetworkMap)
		}

		result.PodCIDRs = append(result.PodCIDRs, cidr)
	}

	serviceNetworks, found, err := unstructured.NestedSlice(cr.Object, "spec", "serviceNetwork")
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving spec.serviceNetwork field")
	} else if !found {
		return nil, fmt.Errorf("field .spec.serviceNetwork expected, but not found in Network resource: %v", cr.Object)
	}

	for _, serviceNetwork := range serviceNetworks {
		result.ServiceCIDRs = append(result.ServiceCIDRs, serviceNetwork.(string))
	}

	ocpNetworkType, found, err := unstructured.NestedString(cr.Object, "spec", "networkType")
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving spec.networkType field")
	} else if !found {
		return nil, fmt.Errorf("field .spec.networkType expected, but not found in Network resource: %v", cr.Object)
	}

	if ocpNetworkType == "Calico" {
		result.NetworkPlugin = cni.Calico
	} else {
		result.NetworkPlugin = ocpNetworkType
	}

	return result, nil
}
