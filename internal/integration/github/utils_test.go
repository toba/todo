package github

import (
	"errors"
	"testing"
)

func TestIsTransientNetworkError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "stream error",
			err:  errors.New("stream error"),
			want: true,
		},
		{
			name: "INTERNAL_ERROR",
			err:  errors.New("INTERNAL_ERROR"),
			want: true,
		},
		{
			name: "connection reset",
			err:  errors.New("connection reset"),
			want: true,
		},
		{
			name: "connection refused",
			err:  errors.New("connection refused"),
			want: true,
		},
		{
			name: "EOF",
			err:  errors.New("EOF"),
			want: true,
		},
		{
			name: "timeout",
			err:  errors.New("timeout"),
			want: true,
		},
		{
			name: "Timeout",
			err:  errors.New("Timeout"),
			want: true,
		},
		{
			name: "some other error",
			err:  errors.New("some other error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientNetworkError(tt.err)
			if got != tt.want {
				t.Errorf("isTransientNetworkError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsTransientHTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       bool
	}{
		{
			name:       "500 empty body",
			statusCode: 500,
			body:       []byte{},
			want:       true,
		},
		{
			name:       "502 empty body",
			statusCode: 502,
			body:       []byte{},
			want:       true,
		},
		{
			name:       "503 empty body",
			statusCode: 503,
			body:       []byte{},
			want:       true,
		},
		{
			name:       "504 empty body",
			statusCode: 504,
			body:       []byte{},
			want:       true,
		},
		{
			name:       "502 with try again",
			statusCode: 502,
			body:       []byte("please try again later"),
			want:       true,
		},
		{
			name:       "502 with Try again",
			statusCode: 502,
			body:       []byte("Try again in a few minutes"),
			want:       true,
		},
		{
			name:       "200 empty body",
			statusCode: 200,
			body:       []byte{},
			want:       false,
		},
		{
			name:       "429 empty body",
			statusCode: 429,
			body:       []byte{},
			want:       false,
		},
		{
			name:       "400 empty body",
			statusCode: 400,
			body:       []byte{},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientHTTPError(tt.statusCode, tt.body)
			if got != tt.want {
				t.Errorf("isTransientHTTPError(%d, %q) = %v, want %v", tt.statusCode, tt.body, got, tt.want)
			}
		})
	}
}
