/*
Copyright 2024 The Kubernetes Authors.

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
package networking

import (
	"reflect"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/golang/mock/gomock"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	infrav1 "sigs.k8s.io/cluster-api-provider-openstack/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-openstack/pkg/clients/mock"
	"sigs.k8s.io/cluster-api-provider-openstack/pkg/scope"
)

func TestValidateRemoteManagedGroups(t *testing.T) {
	tests := []struct {
		name                string
		rule                infrav1.SecurityGroupRuleSpec
		remoteManagedGroups map[string]string
		wantErr             bool
	}{
		{
			name: "Invalid rule with unknown remoteManagedGroup",
			rule: infrav1.SecurityGroupRuleSpec{
				RemoteManagedGroups: []infrav1.ManagedSecurityGroupName{"unknownGroup"},
			},
			wantErr: true,
		},
		{
			name: "Valid rule with missing remoteManagedGroups",
			rule: infrav1.SecurityGroupRuleSpec{
				PortRangeMin: pointer.Int(22),
				PortRangeMax: pointer.Int(22),
				Protocol:     pointer.String("tcp"),
			},
			remoteManagedGroups: map[string]string{
				"self":         "self",
				"controlplane": "1",
				"worker":       "2",
				"bastion":      "3",
			},
			wantErr: true,
		},
		{
			name: "Valid rule with remoteManagedGroups",
			rule: infrav1.SecurityGroupRuleSpec{
				RemoteManagedGroups: []infrav1.ManagedSecurityGroupName{"controlplane", "worker", "bastion"},
			},
			remoteManagedGroups: map[string]string{
				"self":         "self",
				"controlplane": "1",
				"worker":       "2",
				"bastion":      "3",
			},
			wantErr: false,
		},
		{
			name: "Invalid rule with bastion in remoteManagedGroups",
			rule: infrav1.SecurityGroupRuleSpec{
				RemoteManagedGroups: []infrav1.ManagedSecurityGroupName{"controlplane", "worker", "bastion"},
			},
			remoteManagedGroups: map[string]string{
				"self":         "self",
				"controlplane": "1",
				"worker":       "2",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRemoteManagedGroups(tt.remoteManagedGroups, tt.rule.RemoteManagedGroups)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAllNodesRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetAllNodesRules(t *testing.T) {
	tests := []struct {
		name                       string
		remoteManagedGroups        map[string]string
		allNodesSecurityGroupRules []infrav1.SecurityGroupRuleSpec
		wantRules                  []resolvedSecurityGroupRuleSpec
		wantErr                    bool
	}{
		{
			name:                       "Empty remoteManagedGroups and allNodesSecurityGroupRules",
			remoteManagedGroups:        map[string]string{},
			allNodesSecurityGroupRules: []infrav1.SecurityGroupRuleSpec{},
			wantRules:                  []resolvedSecurityGroupRuleSpec{},
			wantErr:                    false,
		},
		{
			name: "Valid remoteManagedGroups and allNodesSecurityGroupRules",
			remoteManagedGroups: map[string]string{
				"controlplane": "1",
				"worker":       "2",
			},
			allNodesSecurityGroupRules: []infrav1.SecurityGroupRuleSpec{
				{
					Protocol:     pointer.String("tcp"),
					PortRangeMin: pointer.Int(22),
					PortRangeMax: pointer.Int(22),
					RemoteManagedGroups: []infrav1.ManagedSecurityGroupName{
						"controlplane",
						"worker",
					},
				},
			},
			wantRules: []resolvedSecurityGroupRuleSpec{
				{
					Protocol:      "tcp",
					PortRangeMin:  22,
					PortRangeMax:  22,
					RemoteGroupID: "1",
				},
				{
					Protocol:      "tcp",
					PortRangeMin:  22,
					PortRangeMax:  22,
					RemoteGroupID: "2",
				},
			},
			wantErr: false,
		},
		{
			name: "Valid remoteManagedGroups in a rule",
			remoteManagedGroups: map[string]string{
				"controlplane": "1",
				"worker":       "2",
			},
			allNodesSecurityGroupRules: []infrav1.SecurityGroupRuleSpec{
				{
					Protocol:            pointer.String("tcp"),
					PortRangeMin:        pointer.Int(22),
					PortRangeMax:        pointer.Int(22),
					RemoteManagedGroups: []infrav1.ManagedSecurityGroupName{"controlplane"},
				},
			},
			wantRules: []resolvedSecurityGroupRuleSpec{
				{
					Protocol:      "tcp",
					PortRangeMin:  22,
					PortRangeMax:  22,
					RemoteGroupID: "1",
				},
			},
		},
		{
			name: "Invalid allNodesSecurityGroupRules with wrong remoteManagedGroups",
			remoteManagedGroups: map[string]string{
				"controlplane": "1",
				"worker":       "2",
			},
			allNodesSecurityGroupRules: []infrav1.SecurityGroupRuleSpec{
				{
					Protocol:     pointer.String("tcp"),
					PortRangeMin: pointer.Int(22),
					PortRangeMax: pointer.Int(22),
					RemoteManagedGroups: []infrav1.ManagedSecurityGroupName{
						"controlplanezzz",
						"worker",
					},
				},
			},
			wantRules: nil,
			wantErr:   true,
		},
		{
			name: "Invalid allNodesSecurityGroupRules with bastion while remoteManagedGroups does not have bastion",
			remoteManagedGroups: map[string]string{
				"controlplane": "1",
				"worker":       "2",
			},
			allNodesSecurityGroupRules: []infrav1.SecurityGroupRuleSpec{
				{
					Protocol:     pointer.String("tcp"),
					PortRangeMin: pointer.Int(22),
					PortRangeMax: pointer.Int(22),
					RemoteManagedGroups: []infrav1.ManagedSecurityGroupName{
						"bastion",
					},
				},
			},
			wantRules: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRules, err := getAllNodesRules(tt.remoteManagedGroups, tt.allNodesSecurityGroupRules)
			if (err != nil) != tt.wantErr {
				t.Errorf("getAllNodesRules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRules, tt.wantRules) {
				t.Errorf("getAllNodesRules() gotRules = %v, want %v", gotRules, tt.wantRules)
			}
		})
	}
}

func TestGenerateDesiredSecGroups(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	secGroupNames := map[string]string{
		"controlplane": "k8s-cluster-mycluster-secgroup-controlplane",
		"worker":       "k8s-cluster-mycluster-secgroup-worker",
	}

	tests := []struct {
		name             string
		openStackCluster *infrav1.OpenStackCluster
		mockExpect       func(m *mock.MockNetworkClientMockRecorder)
		// We could also test the exact rules that are returned, but that'll be a lot data to write out.
		// For now we just make sure that the number of rules is correct.
		expectedNumberSecurityGroupRules int
		wantErr                          bool
	}{
		{
			name:                             "Valid openStackCluster with unmanaged securityGroups",
			openStackCluster:                 &infrav1.OpenStackCluster{},
			mockExpect:                       func(m *mock.MockNetworkClientMockRecorder) {},
			expectedNumberSecurityGroupRules: 0,
			wantErr:                          false,
		},
		{
			name: "Valid openStackCluster with securityGroups",
			openStackCluster: &infrav1.OpenStackCluster{
				Spec: infrav1.OpenStackClusterSpec{
					ManagedSecurityGroups: &infrav1.ManagedSecurityGroups{},
				},
			},
			mockExpect: func(m *mock.MockNetworkClientMockRecorder) {
				m.ListSecGroup(groups.ListOpts{Name: "k8s-cluster-mycluster-secgroup-controlplane"}).Return([]groups.SecGroup{
					{
						ID:   "0",
						Name: "k8s-cluster-mycluster-secgroup-controlplane",
					},
				}, nil)
				m.ListSecGroup(groups.ListOpts{Name: "k8s-cluster-mycluster-secgroup-worker"}).Return([]groups.SecGroup{
					{
						ID:   "1",
						Name: "k8s-cluster-mycluster-secgroup-worker",
					},
				}, nil)
			},
			expectedNumberSecurityGroupRules: 12,
			wantErr:                          false,
		},
		{
			name: "Valid openStackCluster with securityGroups and allNodesSecurityGroupRules",
			openStackCluster: &infrav1.OpenStackCluster{
				Spec: infrav1.OpenStackClusterSpec{
					ManagedSecurityGroups: &infrav1.ManagedSecurityGroups{
						AllNodesSecurityGroupRules: []infrav1.SecurityGroupRuleSpec{
							{
								Protocol:            pointer.String("tcp"),
								PortRangeMin:        pointer.Int(22),
								PortRangeMax:        pointer.Int(22),
								RemoteManagedGroups: []infrav1.ManagedSecurityGroupName{"controlplane", "worker"},
							},
						},
					},
				},
			},
			mockExpect: func(m *mock.MockNetworkClientMockRecorder) {
				m.ListSecGroup(groups.ListOpts{Name: "k8s-cluster-mycluster-secgroup-controlplane"}).Return([]groups.SecGroup{
					{
						ID:   "0",
						Name: "k8s-cluster-mycluster-secgroup-controlplane",
					},
				}, nil)
				m.ListSecGroup(groups.ListOpts{Name: "k8s-cluster-mycluster-secgroup-worker"}).Return([]groups.SecGroup{
					{
						ID:   "1",
						Name: "k8s-cluster-mycluster-secgroup-worker",
					},
				}, nil)
			},
			expectedNumberSecurityGroupRules: 16,
			wantErr:                          false,
		},
		{
			name: "Valid openStackCluster with securityGroups with invalid allNodesSecurityGroupRules",
			openStackCluster: &infrav1.OpenStackCluster{
				Spec: infrav1.OpenStackClusterSpec{
					ManagedSecurityGroups: &infrav1.ManagedSecurityGroups{
						AllNodesSecurityGroupRules: []infrav1.SecurityGroupRuleSpec{
							{
								Protocol:            pointer.String("tcp"),
								PortRangeMin:        pointer.Int(22),
								PortRangeMax:        pointer.Int(22),
								RemoteManagedGroups: []infrav1.ManagedSecurityGroupName{"controlplane", "worker", "unknownGroup"},
							},
						},
					},
				},
			},
			mockExpect: func(m *mock.MockNetworkClientMockRecorder) {
				m.ListSecGroup(groups.ListOpts{Name: "k8s-cluster-mycluster-secgroup-controlplane"}).Return([]groups.SecGroup{
					{
						ID:   "0",
						Name: "k8s-cluster-mycluster-secgroup-controlplane",
					},
				}, nil)
				m.ListSecGroup(groups.ListOpts{Name: "k8s-cluster-mycluster-secgroup-worker"}).Return([]groups.SecGroup{
					{
						ID:   "1",
						Name: "k8s-cluster-mycluster-secgroup-worker",
					},
				}, nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			log := testr.New(t)
			mockScopeFactory := scope.NewMockScopeFactory(mockCtrl, "")

			s, err := NewService(scope.NewWithLogger(mockScopeFactory, log))
			if err != nil {
				t.Fatalf("Failed to create service: %v", err)
			}
			tt.mockExpect(mockScopeFactory.NetworkClient.EXPECT())

			gotSecurityGroups, err := s.generateDesiredSecGroups(tt.openStackCluster, secGroupNames)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			var gotNumberSecurityGroupRules int
			for _, secGroup := range gotSecurityGroups {
				gotNumberSecurityGroupRules += len(secGroup.Rules)
			}
			g.Expect(gotNumberSecurityGroupRules).To(Equal(tt.expectedNumberSecurityGroupRules))
		})
	}
}

func TestReconcileGroupRules(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name             string
		desiredSGSpecs   securityGroupSpec
		observedSGStatus infrav1.SecurityGroupStatus
		mockExpect       func(m *mock.MockNetworkClientMockRecorder)
		wantSGStatus     infrav1.SecurityGroupStatus
	}{
		{
			name:             "Empty desiredSGSpecs and observedSGStatus",
			desiredSGSpecs:   securityGroupSpec{},
			observedSGStatus: infrav1.SecurityGroupStatus{},
			mockExpect:       func(m *mock.MockNetworkClientMockRecorder) {},
			wantSGStatus:     infrav1.SecurityGroupStatus{},
		},
		{
			name: "Same desiredSGSpecs and observedSGStatus produces no changes",
			desiredSGSpecs: securityGroupSpec{
				Name: "k8s-cluster-mycluster-secgroup-controlplane",
				Rules: []resolvedSecurityGroupRuleSpec{
					{
						Description:    "Allow SSH",
						Direction:      "ingress",
						EtherType:      "IPv4",
						Protocol:       "tcp",
						PortRangeMin:   22,
						PortRangeMax:   22,
						RemoteGroupID:  "1",
						RemoteIPPrefix: "",
					},
				},
			},
			observedSGStatus: infrav1.SecurityGroupStatus{
				ID:   "idSG",
				Name: "k8s-cluster-mycluster-secgroup-controlplane",
				Rules: []infrav1.SecurityGroupRuleStatus{
					{
						Description:    pointer.String("Allow SSH"),
						Direction:      "ingress",
						EtherType:      pointer.String("IPv4"),
						ID:             "idSGRule",
						Protocol:       pointer.String("tcp"),
						PortRangeMin:   pointer.Int(22),
						PortRangeMax:   pointer.Int(22),
						RemoteGroupID:  pointer.String("1"),
						RemoteIPPrefix: pointer.String(""),
					},
				},
			},
			mockExpect: func(m *mock.MockNetworkClientMockRecorder) {},
			wantSGStatus: infrav1.SecurityGroupStatus{
				ID:   "idSG",
				Name: "k8s-cluster-mycluster-secgroup-controlplane",
				Rules: []infrav1.SecurityGroupRuleStatus{
					{
						Description:    pointer.String("Allow SSH"),
						Direction:      "ingress",
						EtherType:      pointer.String("IPv4"),
						ID:             "idSGRule",
						Protocol:       pointer.String("tcp"),
						PortRangeMin:   pointer.Int(22),
						PortRangeMax:   pointer.Int(22),
						RemoteGroupID:  pointer.String("1"),
						RemoteIPPrefix: pointer.String(""),
					},
				},
			},
		},
		{
			name: "Different desiredSGSpecs and observedSGStatus produces changes",
			desiredSGSpecs: securityGroupSpec{
				Name: "k8s-cluster-mycluster-secgroup-controlplane",
				Rules: []resolvedSecurityGroupRuleSpec{
					{
						Description:    "Allow SSH",
						Direction:      "ingress",
						EtherType:      "IPv4",
						Protocol:       "tcp",
						PortRangeMin:   22,
						PortRangeMax:   22,
						RemoteGroupID:  "1",
						RemoteIPPrefix: "",
					},
				},
			},
			observedSGStatus: infrav1.SecurityGroupStatus{
				ID:   "idSG",
				Name: "k8s-cluster-mycluster-secgroup-controlplane",
				Rules: []infrav1.SecurityGroupRuleStatus{
					{
						Description:    pointer.String("Allow SSH legacy"),
						Direction:      "ingress",
						EtherType:      pointer.String("IPv4"),
						ID:             "idSGRuleLegacy",
						Protocol:       pointer.String("tcp"),
						PortRangeMin:   pointer.Int(222),
						PortRangeMax:   pointer.Int(222),
						RemoteGroupID:  pointer.String("2"),
						RemoteIPPrefix: pointer.String(""),
					},
				},
			},
			mockExpect: func(m *mock.MockNetworkClientMockRecorder) {
				m.DeleteSecGroupRule("idSGRuleLegacy").Return(nil)
				m.CreateSecGroupRule(rules.CreateOpts{
					SecGroupID:    "idSG",
					Description:   "Allow SSH",
					Direction:     "ingress",
					EtherType:     "IPv4",
					Protocol:      "tcp",
					PortRangeMin:  22,
					PortRangeMax:  22,
					RemoteGroupID: "1",
				}).Return(&rules.SecGroupRule{
					ID:            "idSGRule",
					Description:   "Allow SSH",
					Direction:     "ingress",
					EtherType:     "IPv4",
					Protocol:      "tcp",
					PortRangeMin:  22,
					PortRangeMax:  22,
					RemoteGroupID: "1",
				}, nil)
			},
			wantSGStatus: infrav1.SecurityGroupStatus{
				ID:   "idSG",
				Name: "k8s-cluster-mycluster-secgroup-controlplane",
				Rules: []infrav1.SecurityGroupRuleStatus{
					{
						Description:    pointer.String("Allow SSH"),
						Direction:      "ingress",
						EtherType:      pointer.String("IPv4"),
						ID:             "idSGRule",
						Protocol:       pointer.String("tcp"),
						PortRangeMin:   pointer.Int(22),
						PortRangeMax:   pointer.Int(22),
						RemoteGroupID:  pointer.String("1"),
						RemoteIPPrefix: pointer.String(""),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			log := testr.New(t)
			mockScopeFactory := scope.NewMockScopeFactory(mockCtrl, "")

			s, err := NewService(scope.NewWithLogger(mockScopeFactory, log))
			if err != nil {
				t.Fatalf("Failed to create service: %v", err)
			}
			tt.mockExpect(mockScopeFactory.NetworkClient.EXPECT())

			sgStatus, err := s.reconcileGroupRules(tt.desiredSGSpecs, tt.observedSGStatus)
			g.Expect(err).To(BeNil())
			g.Expect(sgStatus).To(Equal(tt.wantSGStatus))
		})
	}
}
