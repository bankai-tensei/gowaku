package theme

// WithExtension returns a copy of th that also holds v, a value of the
// application's own tokens, which the standard ones do not cover. It replaces a
// value of type T that th already holds (for an interface T, the first that
// implements it); th itself is left as it was, so that a
// shared Theme such as those of Default stays read-only. Read the value back
// with Extension or ExtensionOr. T is usually a struct:
//
//	type Brand struct{ Highlight geom.Color }
//
//	th = theme.WithExtension(th, Brand{Highlight: geom.Hex(0xffd60a)})
//
// A theme holds one brightness, and so do its extensions; WithExtensionPair
// gives a Set its light and dark values at once. A nil v panics: it has no type
// to be found by.
func WithExtension[T any](th *Theme, v T) *Theme {
	if any(v) == nil {
		panic("gowaku: theme.WithExtension given a nil value, which has no type to look it up by")
	}
	out := *th
	out.extensions = make([]any, len(th.extensions), len(th.extensions)+1)
	copy(out.extensions, th.extensions)
	for i, e := range out.extensions {
		// A type assertion to T tells the types apart without reflect.
		if _, ok := e.(T); ok {
			out.extensions[i] = v
			return &out
		}
	}
	out.extensions = append(out.extensions, v)
	return &out
}

// WithExtensionPair returns s with light added to its light theme and dark to
// its dark one, as WithExtension adds them. A missing theme is filled in first,
// as For would fill it in.
//
//	set := theme.FromAccent(accent)
//	set = theme.WithExtensionPair(set, Brand{Highlight: geom.Hex(0xffd60a)}, Brand{Highlight: geom.Hex(0x806b05)})
func WithExtensionPair[T any](s Set, light, dark T) Set {
	return Set{
		Light: WithExtension(s.For(BrightnessLight), light),
		Dark:  WithExtension(s.For(BrightnessDark), dark),
	}
}

// Extension returns the value of type T that th holds, and whether it holds
// one. An interface type finds the first value that implements it. A nil th
// holds nothing.
//
// It is meant for Build, which runs every frame: it scans a short slice with
// type assertions and does not allocate.
func Extension[T any](th *Theme) (T, bool) {
	if th != nil {
		for _, e := range th.extensions {
			if v, ok := e.(T); ok {
				return v, true
			}
		}
	}
	var zero T
	return zero, false
}

// ExtensionOr returns the value of type T that th holds, or fallback when it
// holds none.
func ExtensionOr[T any](th *Theme, fallback T) T {
	if v, ok := Extension[T](th); ok {
		return v
	}
	return fallback
}
