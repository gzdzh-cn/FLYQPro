package service

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"flyqpro/internal/chat"
)

func TestParsePreviewRange(t *testing.T) {
	tests := []struct {
		name         string
		header       string
		wantStart    int64
		wantEnd      int64
		wantSuffix   int64
		wantStartSet bool
		wantEndSet   bool
		wantErr      bool
	}{
		{name: "empty", header: ""},
		{name: "open ended", header: "bytes=1024-", wantStart: 1024, wantEnd: -1, wantSuffix: -1, wantStartSet: true},
		{name: "bounded", header: "bytes=10-20", wantStart: 10, wantEnd: 20, wantSuffix: -1, wantStartSet: true, wantEndSet: true},
		{name: "suffix", header: "bytes=-500", wantStart: -1, wantEnd: -1, wantSuffix: 500},
		{name: "multiple", header: "bytes=0-10,20-30", wantErr: true},
		{name: "invalid", header: "bytes=20-10", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parsePreviewRange(test.header)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				if test.header != "" {
					t.Fatal("expected a range")
				}
				return
			}
			if got.start != test.wantStart || got.end != test.wantEnd || got.suffix != test.wantSuffix || got.hasStart != test.wantStartSet || got.hasEnd != test.wantEndSet {
				t.Fatalf("range = %+v", got)
			}
		})
	}
}

func TestResolvePreviewRange(t *testing.T) {
	bounded, err := parsePreviewRange("bytes=10-20")
	if err != nil {
		t.Fatal(err)
	}
	start, end, ok := resolvePreviewRange(bounded, 100)
	if !ok || start != 10 || end != 20 {
		t.Fatalf("resolved bounded range = %d-%d, %t", start, end, ok)
	}

	suffix, err := parsePreviewRange("bytes=-10")
	if err != nil {
		t.Fatal(err)
	}
	start, end, ok = resolvePreviewRange(suffix, 100)
	if !ok || start != 90 || end != 99 {
		t.Fatalf("resolved suffix range = %d-%d, %t", start, end, ok)
	}

	invalid, err := parsePreviewRange("bytes=100-")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok = resolvePreviewRange(invalid, 100); ok {
		t.Fatal("expected an unsatisfiable range")
	}
}

func previewTestEntry(size int64) chat.SharedEntry {
	return chat.SharedEntry{
		Name:     "movie.mp4",
		MimeType: "video/mp4",
		Size:     size,
	}
}

type previewPipeListener struct {
	connections chan net.Conn
	closed      chan struct{}
	closeOnce   sync.Once
}

func newPreviewPipeListener(conn net.Conn) *previewPipeListener {
	listener := &previewPipeListener{
		connections: make(chan net.Conn, 1),
		closed:      make(chan struct{}),
	}
	listener.connections <- conn
	return listener
}

func (l *previewPipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connections:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *previewPipeListener) Close() error {
	l.closeOnce.Do(func() { close(l.closed) })
	return nil
}

func (l *previewPipeListener) Addr() net.Addr { return previewPipeAddr("preview") }

type previewPipeAddr string

func (a previewPipeAddr) Network() string { return "preview-pipe" }
func (a previewPipeAddr) String() string  { return string(a) }

func TestServeFriendPreviewStreamsBeforeCompletion(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/preview", nil)
	firstWritten := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseStream := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseStream)
	done := make(chan struct{})

	go func() {
		serveFriendPreview(recorder, request, previewToken{}, func(
			_ context.Context,
			_ string,
			_ string,
			_ string,
			_ int64,
			before func(chat.SharedEntry) error,
			write func([]byte) error,
		) error {
			if err := before(previewTestEntry(11)); err != nil {
				return err
			}
			if err := write([]byte("first")); err != nil {
				return err
			}
			close(firstWritten)
			<-release
			return write([]byte("second"))
		}, nil)
		close(done)
	}()

	select {
	case <-firstWritten:
	case <-time.After(time.Second):
		t.Fatal("首块数据没有在传输结束前写入 HTTP 响应")
	}
	if got := recorder.Body.String(); got != "first" {
		t.Fatalf("首块响应 = %q, want %q", got, "first")
	}
	select {
	case <-done:
		t.Fatal("流在收到第二块数据前意外结束")
	default:
	}
	releaseStream()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("流没有正常结束")
	}
	if got := recorder.Body.String(); got != "firstsecond" {
		t.Fatalf("完整响应 = %q", got)
	}
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Length") != "11" {
		t.Fatalf("响应状态/长度 = %d/%q", recorder.Code, recorder.Header().Get("Content-Length"))
	}
}

func TestServeFriendPreviewFlushesFirstChunkOverHTTP(t *testing.T) {
	firstWritten := make(chan struct{})
	streamDone := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseStream := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseStream()

	stream := func(
		ctx context.Context,
		_ string,
		_ string,
		_ string,
		_ int64,
		before func(chat.SharedEntry) error,
		write func([]byte) error,
	) error {
		if err := before(previewTestEntry(11)); err != nil {
			return err
		}
		if err := write([]byte("first")); err != nil {
			return err
		}
		close(firstWritten)
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		if err := write([]byte("second")); err != nil {
			return err
		}
		close(streamDone)
		return nil
	}

	serverConn, clientConn := net.Pipe()
	listener := newPreviewPipeListener(serverConn)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveFriendPreview(w, r, previewToken{}, stream, nil)
	})}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		_ = server.Close()
		_ = clientConn.Close()
	}()
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				return clientConn, nil
			},
		},
	}

	type responseResult struct {
		response *http.Response
		err      error
	}
	responseChannel := make(chan responseResult, 1)
	go func() {
		response, requestErr := client.Get("http://preview-pipe/preview")
		responseChannel <- responseResult{response: response, err: requestErr}
	}()
	var response *http.Response
	select {
	case result := <-responseChannel:
		if result.err != nil {
			t.Fatal(result.err)
		}
		response = result.response
	case <-time.After(time.Second):
		t.Fatal("HTTP 响应头没有在首块数据到达前返回")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("HTTP 状态 = %d, want %d", response.StatusCode, http.StatusOK)
	}
	firstResult := make(chan error, 1)
	go func() {
		first := make([]byte, len("first"))
		_, readErr := io.ReadFull(response.Body, first)
		if readErr == nil && string(first) != "first" {
			readErr = errors.New("首块内容不正确")
		}
		firstResult <- readErr
	}()
	select {
	case <-streamDone:
		t.Fatal("HTTP 客户端读到首块前，发送端已经完成整个流")
	case err := <-firstResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("HTTP 客户端没有读到首块数据")
	}

	select {
	case <-firstWritten:
	default:
		t.Fatal("发送端没有写出首块数据")
	}
	releaseStream()
	rest, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(rest) != "second" {
		t.Fatalf("剩余响应 = %q, want %q", rest, "second")
	}
	select {
	case <-streamDone:
	case <-time.After(time.Second):
		t.Fatal("HTTP 流没有正常结束")
	}
}

func TestServeFriendPreviewHeadDoesNotStartStream(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodHead, "/preview", nil)
	streamCalls := 0
	detailsCalls := 0
	serveFriendPreview(recorder, request, previewToken{}, func(
		context.Context,
		string,
		string,
		string,
		int64,
		func(chat.SharedEntry) error,
		func([]byte) error,
	) error {
		streamCalls++
		return nil
	}, func(string, string, string) (chat.SharedEntry, error) {
		detailsCalls++
		return previewTestEntry(2048), nil
	})

	if streamCalls != 0 {
		t.Fatalf("HEAD 启动了 %d 次数据流", streamCalls)
	}
	if detailsCalls != 1 {
		t.Fatalf("HEAD 元数据请求次数 = %d, want 1", detailsCalls)
	}
	if recorder.Code != http.StatusOK || recorder.Body.Len() != 0 {
		t.Fatalf("HEAD 响应 = status %d, body %d", recorder.Code, recorder.Body.Len())
	}
	if got := recorder.Header().Get("Content-Length"); got != "2048" {
		t.Fatalf("HEAD Content-Length = %q", got)
	}
}

func TestServeFriendPreviewRanges(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantOffset int64
		wantRange  string
		wantBody   string
	}{
		{name: "bounded", header: "bytes=10-20", wantOffset: 10, wantRange: "bytes 10-20/100", wantBody: "abcdefghijk"},
		{name: "suffix", header: "bytes=-10", wantOffset: 90, wantRange: "bytes 90-99/100", wantBody: "abcdefghij"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/preview", nil)
			request.Header.Set("Range", test.header)
			var gotOffset int64
			stream := func(
				_ context.Context,
				_ string,
				_ string,
				_ string,
				offset int64,
				before func(chat.SharedEntry) error,
				write func([]byte) error,
			) error {
				gotOffset = offset
				if err := before(previewTestEntry(100)); err != nil {
					return err
				}
				err := write([]byte("abcdefghijklmnopqrstuvwxyz"))
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
			details := func(string, string, string) (chat.SharedEntry, error) {
				return previewTestEntry(100), nil
			}
			serveFriendPreview(recorder, request, previewToken{}, stream, details)
			if gotOffset != test.wantOffset {
				t.Fatalf("流偏移 = %d, want %d", gotOffset, test.wantOffset)
			}
			if recorder.Code != http.StatusPartialContent {
				t.Fatalf("响应状态 = %d", recorder.Code)
			}
			if got := recorder.Header().Get("Content-Range"); got != test.wantRange {
				t.Fatalf("Content-Range = %q, want %q", got, test.wantRange)
			}
			if got := recorder.Body.String(); got != test.wantBody {
				t.Fatalf("响应内容 = %q, want %q", got, test.wantBody)
			}
		})
	}
}
