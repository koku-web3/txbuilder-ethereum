package errors

import (
	"errors"
	"fmt"
)

func New(text string) error {
	return errors.New(text)
}

// Wrap 将错误包装上层的上下文信息，返回一个新的错误。
// 使用 %w 格式化动词来保持错误链，方便使用 errors.Is / errors.As 检查。
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
