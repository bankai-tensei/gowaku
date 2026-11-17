package golden

// controlScenes returns the scenes of the display, choice, transient, field
// and picker widgets. Each group lives in its own file so that the groups can
// be written apart; the order here is fixed, as the scenes share one Renderer.
func controlScenes() []Scene {
	s := append([]Scene(nil), displayScenes()...)
	s = append(s, choiceScenes()...)
	s = append(s, transientScenes()...)
	s = append(s, fieldScenes()...)
	s = append(s, pickerScenes()...)
	return s
}

// desktopScenes returns the scenes of the table, tree, tab bar, editor and pane
// widgets, in a fixed order as controlScenes does.
func desktopScenes() []Scene {
	s := append([]Scene(nil), tableScenes()...)
	s = append(s, treeScenes()...)
	s = append(s, tabBarScenes()...)
	s = append(s, editorScenes()...)
	s = append(s, paneScenes()...)
	return s
}

// appScenes returns the scenes of the navigator, the localized strings and the
// widgets no other scene covers, in a fixed order as controlScenes does.
func appScenes() []Scene {
	s := append([]Scene(nil), navigatorScenes()...)
	s = append(s, localeScenes()...)
	s = append(s, coverageScenes()...)
	return s
}
