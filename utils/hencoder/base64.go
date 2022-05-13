package hencoder

import "encoding/base64"

func Base64DecodeString(str string) (string, error) {
	p, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", err
	}
	return string(p), err
}

func Base64Decode(bs []byte) ([]byte, error) {
	var res []byte
	_, err := base64.StdEncoding.Decode(bs, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func Base64(bs []byte) string {
	return base64.StdEncoding.EncodeToString(bs)
}
