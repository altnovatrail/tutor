package runner

import (
	"context"

	"github.com/altnovatrail/tutor/provider"
)

// Asker asks a step's request of a model: Models asks Claude, and Stub answers by the markup.
type Asker func(ctx context.Context, st Step, req provider.Request) (provider.Response, error)

// Models asks the models of the steps' configurations with the key, each by the client that the
// adapter dial makes of it once, such as claude.Dial with the Anthropic key: the runner takes the
// adapter from its caller, so that a package of the library that runs its own measurement takes no
// SDK (R90). A model the adapter does not call fails its call, and the run goes on.
func Models(dial provider.Dial, key string) Asker {
	clients := map[string]provider.Model{}
	return func(ctx context.Context, st Step, req provider.Request) (provider.Response, error) {
		c, ok := clients[st.Config.Model.ID]
		if !ok {
			var err error
			if c, err = dial(st.Config.Model.ID, key); err != nil {
				return provider.Response{}, err
			}
			clients[st.Config.Model.ID] = c
		}
		return c.Ask(ctx, req)
	}
}

// Stub answers every call as a model that is always right would, by the markup (Call.Right), with
// the tokens the estimate expects of it, so that a run on it costs what the estimate says. A call
// without such an answer fails, and the run goes on. It asks no one: a run on it needs no key and
// no network.
func Stub(_ context.Context, st Step, _ provider.Request) (provider.Response, error) {
	answer, err := st.Call.Right()
	if err != nil {
		return provider.Response{}, err
	}
	return provider.Response{JSON: answer, Model: st.Config.Model.ID, InputTokens: st.In, OutputTokens: st.Out}, nil
}
