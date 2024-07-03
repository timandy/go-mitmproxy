package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func splitHostPortSource() [][]any {
	return [][]any{
		{"", "", ""},
		{":", "", ""},
		{":80", "", "80"},
		{"clxu6", "clxu6", ""},
		{"clxu6:80", "clxu6", "80"},
		{"www.baidu.com", "www.baidu.com", ""},
		{"www.baidu.com:443", "www.baidu.com", "443"},
		{"[fe80::9d8e:cab8:a33a:2f30%10]", "fe80::9d8e:cab8:a33a:2f30%10", ""},
		{"[fe80::9d8e:cab8:a33a:2f30%10]:443", "fe80::9d8e:cab8:a33a:2f30%10", "443"},
	}
}

func TestSplitHostPort(t *testing.T) {
	for _, line := range splitHostPortSource() {
		raw := line[0].(string)
		host := line[1].(string)
		port := line[2].(string)
		h, p := SplitHostPort(raw)
		assert.Equal(t, host, h)
		assert.Equal(t, port, p)
		h2, p2 := SplitHostPort(h)
		assert.Equal(t, host, h2)
		assert.Equal(t, "", p2)
	}
}

func joinHostPortSource() [][]any {
	return [][]any{
		{"", "", ""},
		{":80", "", "80"},
		{"clxu6", "clxu6", ""},
		{"clxu6:80", "clxu6", "80"},
		{"www.baidu.com", "www.baidu.com", ""},
		{"www.baidu.com:443", "www.baidu.com", "443"},
		{"[fe80::9d8e:cab8:a33a:2f30%10]", "fe80::9d8e:cab8:a33a:2f30%10", ""},
		{"[fe80::9d8e:cab8:a33a:2f30%10]:443", "fe80::9d8e:cab8:a33a:2f30%10", "443"},
	}
}

func TestJoinHostPort(t *testing.T) {
	for _, line := range joinHostPortSource() {
		raw := line[0].(string)
		host := line[1].(string)
		port := line[2].(string)
		hp := JoinHostPort(host, port)
		assert.Equal(t, raw, hp)
	}
}
