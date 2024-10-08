package v1alpha1_test

import (
	"reflect"
	"testing"

	"github.com/argoproj-labs/ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestAccessBinding_RenderSubjects(t *testing.T) {
	app, err := utils.ToUnstructured(&v1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"test": "hello",
			},
		},
	})
	require.NoError(t, err)
	project, err := utils.ToUnstructured(&v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"test": "world",
			},
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name          string
		If            *string
		subjects      []string
		expected      []string
		errorContains string
	}{
		{
			name: "sucessfully return rendered subjects",
			subjects: []string{
				`{{ index .application.metadata.annotations "test" }}`,
				`{{ index .project.metadata.annotations "test" }}`,
			},
			expected: []string{"hello", "world"},
		},
		{
			name:     "return nil if subjects are nil",
			subjects: nil,
			expected: nil,
		},
		{
			name:     "return nil if subjects are empty",
			subjects: []string{},
			expected: nil,
		},
		{
			name:     "return nil if If condition is false",
			If:       ptr.To("false"),
			subjects: []string{"value"},
			expected: nil,
		},
		{
			name:          "return error if If condition is invalid",
			If:            ptr.To("invalid.golang"),
			subjects:      []string{"value"},
			errorContains: "failed to evaluate binding condition",
		},
		{
			name:          "return error if If condition is not a boolean",
			If:            ptr.To("1 + 1"),
			subjects:      []string{"value"},
			errorContains: "evaluated to non-boolean value",
		},
		{
			name:          "return error if subjects template is invalid",
			subjects:      []string{"{{"},
			errorContains: "error parsing AccessBinding subjects",
		},
		{
			name:          "return error if subjects template rendering is invalid",
			subjects:      []string{`{{ index .notAnObject "key" }}`},
			errorContains: "error rendering AccessBinding subjects",
		},
		{
			name:     "can render app as an alias",
			subjects: []string{"{{ .app.metadata.name }}"},
			expected: []string{"test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ab := &api.AccessBinding{
				Spec: api.AccessBindingSpec{
					If:       tt.If,
					Subjects: tt.subjects,
				},
			}
			got, err := ab.RenderSubjects(app, project)
			if err != nil {
				if tt.errorContains != "" {
					assert.ErrorContains(t, err, tt.errorContains)
				} else {
					assert.NoError(t, err)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("AccessBinding.RenderSubjects() = %v, want %v", got, tt.expected)
			}
		})
	}
}
