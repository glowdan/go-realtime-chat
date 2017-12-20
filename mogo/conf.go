package mogo

type Conf struct {
	GraphiteAddr             string
	ForwardingAddr           string
	ForwarderListenAddr      string
	ForwardedNamespace       string
	Port                     int
	DebugPort                int
	DebugLogging             bool
	ClearStatsBetweenFlushes bool
	FlushIntervalMS          int
	Namespace                string
	forwardingEnabled        bool
	forwarderEnabled         bool
}
