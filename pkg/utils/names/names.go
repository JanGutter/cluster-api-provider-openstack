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

package names

import (
	"fmt"
	"strings"
)

const (
	FloatingAddressIPClaimNameSuffix = "floating-ip-address"
)

func GetDescription(clusterName string) string {
	return fmt.Sprintf("Created by cluster-api-provider-openstack cluster %s", clusterName)
}

func GetFloatingAddressClaimName(openStackMachineName string) string {
	return fmt.Sprintf("%s-%s", openStackMachineName, FloatingAddressIPClaimNameSuffix)
}

func GetOpenStackMachineNameFromClaimName(claimName string) string {
	return strings.TrimSuffix(claimName, fmt.Sprintf("-%s", FloatingAddressIPClaimNameSuffix))
}
