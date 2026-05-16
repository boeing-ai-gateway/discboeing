package handler

import (
	"net/http"
	"testing"
)

func TestSudoAuthorizeRequestIsLocal(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       bool
	}{
		{name: "IPv4 loopback", remoteAddr: "127.0.0.1:53210", want: true},
		{name: "IPv6 loopback", remoteAddr: "[::1]:53210", want: true},
		{name: "localhost", remoteAddr: "localhost:53210", want: true},
		{name: "non-loopback", remoteAddr: "192.0.2.10:53210"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{RemoteAddr: tt.remoteAddr}
			if got := sudoAuthorizeRequestIsLocal(req); got != tt.want {
				t.Fatalf("sudoAuthorizeRequestIsLocal() = %v, want %v", got, tt.want)
			}
		})
	}
}
