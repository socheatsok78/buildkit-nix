package builder

import (
	"context"

	"github.com/moby/buildkit/frontend/gateway/client"
)

func solveResultReference(ctx context.Context, c client.Client, req client.SolveRequest) (*client.Result, client.Reference, error) {
	res, err := c.Solve(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return nil, nil, err
	}

	return res, ref, nil
}
