// Package worklist lists Works from the projection for the resume and archive
// pickers and resolves a user-supplied opaque id to a single Work.
//
// List returns active Works (or active plus archived) ordered by
// last_accessed_at descending, with the ULID id breaking ties so the order is
// total. Resolve matches only work.id — never a mutable attribute — and reports
// whether the id was resolved, not found, or refers to an archived Work.
//
// WorkRow is the presentation model both pickers render: an unambiguous name
// (repository name plus slug, with a short-id suffix only when another row
// collides on repo, slug and branch), the branch, and a coarse relative access
// time from internal/reltime.
package worklist
