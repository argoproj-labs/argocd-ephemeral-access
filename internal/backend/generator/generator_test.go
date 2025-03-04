package generator_test

import (
	"testing"

	"github.com/argoproj-labs/argocd-ephemeral-access/internal/backend/generator"
	"github.com/stretchr/testify/assert"
)

func TestToDNS1123Subdomain(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected string
	}{
		{
			name:     "should not contain any invalid char",
			data:     "my.(user)[1234567890] +_)(*&^%$#@!~",
			expected: "my.user1234567890",
		},
		{
			name:     "should not start with a dash",
			data:     "---username",
			expected: "username",
		},
		{
			name:     "should not start with a period",
			data:     "...role",
			expected: "role",
		},
		{
			name:     "should be lowercase",
			data:     "UserName",
			expected: "username",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generator.ToDNS1123Subdomain(tt.data); got != tt.expected {
				t.Errorf("ToDNS1123Subdomain() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestToMaxLength(t *testing.T) {
	type args struct {
		a   string
		b   string
		max int
	}
	tests := []struct {
		name string
		args args
		a    string
		b    string
	}{
		{
			name: "strings already balanced",
			args: args{

				a:   "abcd",
				b:   "1234",
				max: 99,
			},
			a: "abcd",
			b: "1234",
		},
		{
			name: "strings are balanced to max",
			args: args{

				a:   "abcd",
				b:   "1234",
				max: 4,
			},
			a: "ab",
			b: "12",
		},
		{
			name: "remainder to A if max is odd and both too long",
			args: args{

				a:   "abcd",
				b:   "1234",
				max: 5,
			},
			a: "abc",
			b: "12",
		},
		{
			name: "remainder to A if max is odd and A too long",
			args: args{

				a:   "abcd",
				b:   "12",
				max: 5,
			},
			a: "abc",
			b: "12",
		},
		{
			name: "remainder to B if max is odd and B too long",
			args: args{

				a:   "ab",
				b:   "1234",
				max: 5,
			},
			a: "ab",
			b: "123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := generator.ToMaxLength(tt.args.a, tt.args.b, tt.args.max)
			assert.Equal(t, tt.a, a)
			assert.Equal(t, tt.b, b)
			if len(tt.args.a)+len(tt.args.b) >= tt.args.max {
				assert.Equal(t, tt.args.max, len(a)+len(b))
			}
		})
	}
}
