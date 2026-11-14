package pluginadmin

import (
	"context"
	"errors"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplace"
)

// ClassifySourceError projects retrieval, capture and validation failures onto
// the contracted exit categories. Typed errors pass through unchanged and the
// public text never carries transport detail.
func ClassifySourceError(err error) error {
	var typed *diag.Error
	switch {
	case err == nil:
		return nil
	case errors.As(err, &typed):
		return err
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return diag.Wrap(diag.Cancelled, err, "marketplace operation cancelled")
	case errors.Is(err, marketplace.ErrInvalid):
		return diag.Wrap(diag.MarketplaceInvalid, err, safeInvalid(err))
	case errors.Is(err, marketplace.ErrChanged):
		return diag.Wrap(diag.MarketplaceSourceUnavailable, err, "the marketplace source changed while it was being captured; retry when it is stable")
	case errors.Is(err, marketplace.ErrSourceUnavailable):
		return diag.Wrap(diag.MarketplaceSourceUnavailable, err, "the marketplace source, branch or revision is unavailable")
	}
	return err
}

// safeInvalid keeps validation detail (paths and names inside the catalog) but
// drops any wrapped transport cause.
func safeInvalid(err error) string {
	msg := strings.TrimPrefix(err.Error(), marketplace.ErrInvalid.Error()+": ")
	return "the marketplace catalog is invalid: " + msg
}
