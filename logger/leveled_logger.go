package logger

// LeveledLogger is an interface that specifies methods that are needed for
// leveled logging
//
// In the event that you want to import the MetadataFs or TagFs as a library,
// they expect a logger matching this interface
type LeveledLogger interface {
	Fatalf(m string, args ...interface{})
	Debugf(m string, args ...interface{})
	Errorf(m string, args ...interface{})
	Infof(m string, args ...interface{})
	Warningf(m string, args ...interface{})
}
