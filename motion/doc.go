// Package motion holds animation: curves, tweens, springs, the controller that
// runs them, and the bookkeeping of implicit animations.
//
// motion imports core and geom. It must not import gpu or platform. Nothing here
// reads the clock: time comes from the tickers of core, which is what makes the
// values testable. Everything in motion must be used on the UI goroutine.
package motion
