package metadatafs

// Logger is an interface that specifies methods that are needed for leveled logging
type Logger interface {
	Fatalf(m string, args ...interface{})
	Debugf(m string, args ...interface{})
	Errorf(m string, args ...interface{})
	Infof(m string, args ...interface{})
	Warningf(m string, args ...interface{})
}
