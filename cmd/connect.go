package cmd

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"

	"github.com/SeaArt-Infra/sandbox-go/core"
)

const connectFlagEndStream byte = 0x02

type ConnectFrame struct {
	Flags   byte
	Payload json.RawMessage
}

func (f *ConnectFrame) IsEnd() bool {
	return f != nil && f.Flags&connectFlagEndStream != 0
}

func (f *ConnectFrame) Decode(out any) error {
	if len(f.Payload) == 0 {
		return io.EOF
	}
	return json.Unmarshal(f.Payload, out)
}

type ConnectStream struct {
	resp *http.Response
}

func (s *ConnectStream) Response() *http.Response {
	return s.resp
}

func (s *ConnectStream) Close() error {
	if s == nil || s.resp == nil || s.resp.Body == nil {
		return nil
	}
	return s.resp.Body.Close()
}

func (s *ConnectStream) NextFrame() (*ConnectFrame, error) {
	if s == nil || s.resp == nil || s.resp.Body == nil {
		return nil, io.EOF
	}

	header := make([]byte, 5)
	if _, err := io.ReadFull(s.resp.Body, header); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, io.EOF
		}
		return nil, err
	}

	length := binary.BigEndian.Uint32(header[1:])
	payload := make([]byte, length)
	if _, err := io.ReadFull(s.resp.Body, payload); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, io.EOF
		}
		return nil, err
	}

	return &ConnectFrame{Flags: header[0], Payload: payload}, nil
}

func (s *ConnectStream) NextJSON(out any) (*ConnectFrame, error) {
	for {
		frame, err := s.NextFrame()
		if err != nil {
			return nil, err
		}
		if len(frame.Payload) == 0 {
			if frame.IsEnd() {
				return nil, io.EOF
			}
			continue
		}
		if err := json.Unmarshal(frame.Payload, out); err != nil {
			return nil, err
		}
		return frame, nil
	}
}

type ProcessStream struct {
	*ConnectStream
}

func (s *ProcessStream) Next() (*ProcessStreamFrame, error) {
	var frame ProcessStreamFrame
	if _, err := s.NextJSON(&frame); err != nil {
		return nil, err
	}
	return &frame, nil
}

type FilesystemWatchStream struct {
	*ConnectStream
}

func (s *FilesystemWatchStream) Next() (*FilesystemWatchFrame, error) {
	var frame FilesystemWatchFrame
	if _, err := s.NextJSON(&frame); err != nil {
		return nil, err
	}
	return &frame, nil
}

func encodeConnectFrame(payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(make([]byte, 0, 5+len(data)))
	buf.WriteByte(0)
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(data)))
	buf.Write(size[:])
	buf.Write(data)
	return buf.Bytes(), nil
}

func encodeConnectFrames(frames []StreamInputFrame) ([]byte, error) {
	var buf bytes.Buffer
	for _, frame := range frames {
		data, err := encodeConnectFrame(frame)
		if err != nil {
			return nil, err
		}
		buf.Write(data)
	}
	return buf.Bytes(), nil
}

func (c *Service) connectStream(
	ctx context.Context,
	path string,
	body any,
	opts *RequestOptions,
) (*ConnectStream, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(
		ctx,
		http.MethodPost,
		path,
		nil,
		bytes.NewReader(payload),
		"application/connect+json",
		"application/connect+json",
		withBasicUsername(withConnectRPC(opts)),
	)
	if err != nil {
		return nil, err
	}

	// Connect streams can live for the full sandbox session. Keep transport
	// settings, but let the caller context own the stream lifetime.
	streamClient := &http.Client{
		Transport:     c.httpClient.Transport,
		CheckRedirect: c.httpClient.CheckRedirect,
		Jar:           c.httpClient.Jar,
	}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, core.DecodeAPIError(resp)
	}
	return &ConnectStream{resp: resp}, nil
}
