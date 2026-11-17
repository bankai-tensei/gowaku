//go:build windows && !amd64 && !arm64

package com

func floatThunkCode(uintptr, int) ([]byte, error) { return nil, ErrNoThunk }

func floatCallerCode(uintptr, int) ([]byte, error) { return nil, ErrNoThunk }
