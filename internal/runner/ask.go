package runner

import (
	"context"

	"github.com/altnovatrail/tutor/provider"
	"github.com/altnovatrail/tutor/provider/claude"
)

// Asker asks a step's request of a model: Models asks Claude, and Stub answers by the markup.
type Asker func(ctx context.Context, st Step, req provider.Request) (provider.Response, error)

// Models asks the models of the steps' configurations with the Anthropic key.
func Models(key string) Asker {
	clients := map[string]*claude.Client{}
	return func(ctx context.Context, st Step, req provider.Request) (provider.Response, error) {
		c, ok := clients[st.Config.Model.ID]
		if !ok {
			c = claude.New(key, st.Config.Model)
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
