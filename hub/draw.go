package hub

// DrawOptions controls how the screen is drawn.
type DrawOptions struct {
	Prompt       bool // draw only the prompt
	PurgeCache   bool // purge display cache only
	RunningQuery bool // a query is currently running
	DisableCache bool // disable display cache
	ForceSync    bool // force a full screen sync
}
