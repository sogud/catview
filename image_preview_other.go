//go:build !darwin

package main

import (
	"bytes"
	"errors"
	"image"

	"github.com/egoist/mygo/ui"
)

func decodeImagePage(data []byte, page int) (result decodedImage, err error) {
	result.pages = 1
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return result, err
	}
	if config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 16000000 {
		return result, errors.New("image exceeds the preview pixel limit")
	}
	result.picture, err = ui.DecodeBitmap(data)
	return result, err
}
