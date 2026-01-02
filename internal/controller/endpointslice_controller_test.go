package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEndpointSliceReconciler_endpointToRow(t *testing.T) {
	reconciler := &EndpointSliceReconciler{}

	tests := []struct {
		name      string
		ep        *discoveryv1.Endpoint
		namespace string
		service   string
		expected  *endpointRow
	}{
		{
			name: "ready endpoint with pod target ref",
			ep: &discoveryv1.Endpoint{
				Addresses: []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(true),
				},
				TargetRef: &corev1.ObjectReference{
					Kind: "Pod",
					UID:  "pod-uid-123",
					Name: "pod-name-123",
				},
			},
			namespace: "default",
			service:   "my-service",
			expected: &endpointRow{
				UID:  "pod-uid-123",
				Name: "pod-name-123",
				IP:   "10.0.0.1",
			},
		},
		{
			name: "ready endpoint without target ref",
			ep: &discoveryv1.Endpoint{
				Addresses: []string{"10.0.0.2"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(true),
				},
				TargetRef: nil,
			},
			namespace: "default",
			service:   "my-service",
			expected: &endpointRow{
				UID:  "default/my-service/10.0.0.2",
				Name: "",
				IP:   "10.0.0.2",
			},
		},
		{
			name: "ready endpoint with non-pod target ref",
			ep: &discoveryv1.Endpoint{
				Addresses: []string{"10.0.0.3"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(true),
				},
				TargetRef: &corev1.ObjectReference{
					Kind: "Node",
					UID:  "node-uid-123",
					Name: "node-name-123",
				},
			},
			namespace: "default",
			service:   "my-service",
			expected: &endpointRow{
				UID:  "default/my-service/10.0.0.3",
				Name: "",
				IP:   "10.0.0.3",
			},
		},
		{
			name: "not ready endpoint returns nil",
			ep: &discoveryv1.Endpoint{
				Addresses: []string{"10.0.0.4"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(false),
				},
				TargetRef: &corev1.ObjectReference{
					Kind: "Pod",
					UID:  "pod-uid-456",
					Name: "pod-name-456",
				},
			},
			namespace: "default",
			service:   "my-service",
			expected:  nil,
		},
		{
			name: "endpoint with nil ready condition is treated as ready",
			ep: &discoveryv1.Endpoint{
				Addresses: []string{"10.0.0.5"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: nil,
				},
				TargetRef: &corev1.ObjectReference{
					Kind: "Pod",
					UID:  "pod-uid-789",
					Name: "pod-name-789",
				},
			},
			namespace: "default",
			service:   "my-service",
			expected: &endpointRow{
				UID:  "pod-uid-789",
				Name: "pod-name-789",
				IP:   "10.0.0.5",
			},
		},
		{
			name: "endpoint with no addresses returns nil",
			ep: &discoveryv1.Endpoint{
				Addresses: []string{},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(true),
				},
				TargetRef: &corev1.ObjectReference{
					Kind: "Pod",
					UID:  "pod-uid-999",
					Name: "pod-name-999",
				},
			},
			namespace: "default",
			service:   "my-service",
			expected:  nil,
		},
		{
			name: "endpoint with multiple addresses uses first",
			ep: &discoveryv1.Endpoint{
				Addresses: []string{"10.0.0.6", "10.0.0.7"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(true),
				},
				TargetRef: &corev1.ObjectReference{
					Kind: "Pod",
					UID:  "pod-uid-multi",
					Name: "pod-name-multi",
				},
			},
			namespace: "default",
			service:   "my-service",
			expected: &endpointRow{
				UID:  "pod-uid-multi",
				Name: "pod-name-multi",
				IP:   "10.0.0.6",
			},
		},
		{
			name: "endpoint with empty UID in target ref",
			ep: &discoveryv1.Endpoint{
				Addresses: []string{"10.0.0.8"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: boolPtr(true),
				},
				TargetRef: &corev1.ObjectReference{
					Kind: "Pod",
					UID:  "",
					Name: "pod-name-empty-uid",
				},
			},
			namespace: "default",
			service:   "my-service",
			expected: &endpointRow{
				UID:  "default/my-service/10.0.0.8",
				Name: "pod-name-empty-uid",
				IP:   "10.0.0.8",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reconciler.endpointToRow(tt.ep, tt.namespace, tt.service)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("endpointToRow() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("endpointToRow() = nil, want %v", tt.expected)
				} else if *result != *tt.expected {
					t.Errorf("endpointToRow() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestEndpointSliceReconciler_buildDesiredRows(t *testing.T) {
	tests := []struct {
		name          string
		list          *discoveryv1.EndpointSliceList
		service       string
		labelSelector string
		expected      map[string]endpointRow
	}{
		{
			name: "empty list returns empty map",
			list: &discoveryv1.EndpointSliceList{
				Items: []discoveryv1.EndpointSlice{},
			},
			service:       "my-service",
			labelSelector: "",
			expected:      map[string]endpointRow{},
		},
		{
			name: "single endpoint slice with one ready endpoint",
			list: &discoveryv1.EndpointSliceList{
				Items: []discoveryv1.EndpointSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      "slice-1",
							Labels:    map[string]string{},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses: []string{"10.0.0.1"},
								Conditions: discoveryv1.EndpointConditions{
									Ready: boolPtr(true),
								},
								TargetRef: &corev1.ObjectReference{
									Kind: "Pod",
									UID:  "pod-uid-1",
									Name: "pod-name-1",
								},
							},
						},
					},
				},
			},
			service:       "my-service",
			labelSelector: "",
			expected: map[string]endpointRow{
				"pod-uid-1": {
					UID:  "pod-uid-1",
					Name: "pod-name-1",
					IP:   "10.0.0.1",
				},
			},
		},
		{
			name: "multiple endpoint slices with multiple endpoints",
			list: &discoveryv1.EndpointSliceList{
				Items: []discoveryv1.EndpointSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      "slice-1",
							Labels:    map[string]string{},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses: []string{"10.0.0.1"},
								Conditions: discoveryv1.EndpointConditions{
									Ready: boolPtr(true),
								},
								TargetRef: &corev1.ObjectReference{
									Kind: "Pod",
									UID:  "pod-uid-1",
									Name: "pod-name-1",
								},
							},
							{
								Addresses: []string{"10.0.0.2"},
								Conditions: discoveryv1.EndpointConditions{
									Ready: boolPtr(true),
								},
								TargetRef: &corev1.ObjectReference{
									Kind: "Pod",
									UID:  "pod-uid-2",
									Name: "pod-name-2",
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      "slice-2",
							Labels:    map[string]string{},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses: []string{"10.0.0.3"},
								Conditions: discoveryv1.EndpointConditions{
									Ready: boolPtr(true),
								},
								TargetRef: &corev1.ObjectReference{
									Kind: "Pod",
									UID:  "pod-uid-3",
									Name: "pod-name-3",
								},
							},
						},
					},
				},
			},
			service:       "my-service",
			labelSelector: "",
			expected: map[string]endpointRow{
				"pod-uid-1": {
					UID:  "pod-uid-1",
					Name: "pod-name-1",
					IP:   "10.0.0.1",
				},
				"pod-uid-2": {
					UID:  "pod-uid-2",
					Name: "pod-name-2",
					IP:   "10.0.0.2",
				},
				"pod-uid-3": {
					UID:  "pod-uid-3",
					Name: "pod-name-3",
					IP:   "10.0.0.3",
				},
			},
		},
		{
			name: "filters out not ready endpoints",
			list: &discoveryv1.EndpointSliceList{
				Items: []discoveryv1.EndpointSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      "slice-1",
							Labels:    map[string]string{},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses: []string{"10.0.0.1"},
								Conditions: discoveryv1.EndpointConditions{
									Ready: boolPtr(true),
								},
								TargetRef: &corev1.ObjectReference{
									Kind: "Pod",
									UID:  "pod-uid-1",
									Name: "pod-name-1",
								},
							},
							{
								Addresses: []string{"10.0.0.2"},
								Conditions: discoveryv1.EndpointConditions{
									Ready: boolPtr(false),
								},
								TargetRef: &corev1.ObjectReference{
									Kind: "Pod",
									UID:  "pod-uid-2",
									Name: "pod-name-2",
								},
							},
						},
					},
				},
			},
			service:       "my-service",
			labelSelector: "",
			expected: map[string]endpointRow{
				"pod-uid-1": {
					UID:  "pod-uid-1",
					Name: "pod-name-1",
					IP:   "10.0.0.1",
				},
			},
		},
		{
			name: "filters by label selector",
			list: &discoveryv1.EndpointSliceList{
				Items: []discoveryv1.EndpointSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      "slice-1",
							Labels: map[string]string{
								"app": "my-app",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses: []string{"10.0.0.1"},
								Conditions: discoveryv1.EndpointConditions{
									Ready: boolPtr(true),
								},
								TargetRef: &corev1.ObjectReference{
									Kind: "Pod",
									UID:  "pod-uid-1",
									Name: "pod-name-1",
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      "slice-2",
							Labels: map[string]string{
								"app": "other-app",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses: []string{"10.0.0.2"},
								Conditions: discoveryv1.EndpointConditions{
									Ready: boolPtr(true),
								},
								TargetRef: &corev1.ObjectReference{
									Kind: "Pod",
									UID:  "pod-uid-2",
									Name: "pod-name-2",
								},
							},
						},
					},
				},
			},
			service:       "my-service",
			labelSelector: "app=my-app",
			expected: map[string]endpointRow{
				"pod-uid-1": {
					UID:  "pod-uid-1",
					Name: "pod-name-1",
					IP:   "10.0.0.1",
				},
			},
		},
		{
			name: "duplicate UIDs overwrite (last wins)",
			list: &discoveryv1.EndpointSliceList{
				Items: []discoveryv1.EndpointSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      "slice-1",
							Labels:    map[string]string{},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses: []string{"10.0.0.1"},
								Conditions: discoveryv1.EndpointConditions{
									Ready: boolPtr(true),
								},
								TargetRef: &corev1.ObjectReference{
									Kind: "Pod",
									UID:  "pod-uid-1",
									Name: "pod-name-1",
								},
							},
							{
								Addresses: []string{"10.0.0.2"},
								Conditions: discoveryv1.EndpointConditions{
									Ready: boolPtr(true),
								},
								TargetRef: &corev1.ObjectReference{
									Kind: "Pod",
									UID:  "pod-uid-1",
									Name: "pod-name-2",
								},
							},
						},
					},
				},
			},
			service:       "my-service",
			labelSelector: "",
			expected: map[string]endpointRow{
				"pod-uid-1": {
					UID:  "pod-uid-1",
					Name: "pod-name-2",
					IP:   "10.0.0.2",
				},
			},
		},
		{
			name: "endpoints without target ref use generated UID",
			list: &discoveryv1.EndpointSliceList{
				Items: []discoveryv1.EndpointSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      "slice-1",
							Labels:    map[string]string{},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Addresses: []string{"10.0.0.1"},
								Conditions: discoveryv1.EndpointConditions{
									Ready: boolPtr(true),
								},
								TargetRef: nil,
							},
						},
					},
				},
			},
			service:       "my-service",
			labelSelector: "",
			expected: map[string]endpointRow{
				"default/my-service/10.0.0.1": {
					UID:  "default/my-service/10.0.0.1",
					Name: "",
					IP:   "10.0.0.1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &EndpointSliceReconciler{
				LabelSelector: tt.labelSelector,
			}
			result := reconciler.buildDesiredRows(tt.list, tt.service)

			if len(result) != len(tt.expected) {
				t.Errorf("buildDesiredRows() returned %d rows, want %d", len(result), len(tt.expected))
			}

			for uid, expectedRow := range tt.expected {
				actualRow, ok := result[uid]
				if !ok {
					t.Errorf("buildDesiredRows() missing row for UID %q", uid)
					continue
				}
				if actualRow != expectedRow {
					t.Errorf("buildDesiredRows() row for UID %q = %v, want %v", uid, actualRow, expectedRow)
				}
			}
		})
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
