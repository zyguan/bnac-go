package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanMsg(t *testing.T) {
	msg1 := "foo\r\n\r\n"
	msg2 := "bar\r\nhello:world\r\n\r\n"
	s := NewScanner(strings.NewReader(msg1 + msg2))

	assert.True(t, s.Scan())
	assert.Equal(t, []byte(msg1), s.Bytes())
	msg, err := s.Msg()
	assert.Nil(t, err)
	assert.Equal(t, []byte(msg1), msg.Bytes())

	assert.True(t, s.Scan())
	assert.Equal(t, []byte(msg2), s.Bytes())
	msg, err = s.Msg()
	assert.Nil(t, err)
	assert.Equal(t, []byte(msg2), msg.Bytes())

	assert.False(t, s.Scan())
	assert.Nil(t, s.Err())
	msg, err = s.Msg()
	assert.NotNil(t, err)
	assert.Nil(t, msg)
}

func TestRecv(t *testing.T) {
	msg1 := "foo\r\n\r\n"
	msg2 := "bar\r\nhello:world\r\n\r\n"
	s := NewScanner(strings.NewReader(msg1 + msg2))

	msg, err := s.Recv()
	assert.Nil(t, err)
	assert.Equal(t, []byte(msg1), msg.Bytes())

	msg, err = s.Recv()
	assert.Nil(t, err)
	assert.Equal(t, []byte(msg2), msg.Bytes())

	msg, err = s.Recv()
	assert.NotNil(t, err)
}

func TestPutGet(t *testing.T) {
	msg := NewMsg("msg")
	msg.Put("foo", "bar")
	msg.Put("hello", "world")

	v, ok := msg.Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", v)

	v, ok = msg.Get("hello")
	assert.True(t, ok)
	assert.Equal(t, "world", v)

	v, ok = msg.Get("Foo")
	assert.False(t, ok)
	assert.Equal(t, "", v)
}
