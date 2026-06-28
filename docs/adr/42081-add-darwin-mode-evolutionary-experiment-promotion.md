# ADR-42081: Add Darwin Mode for Evolutionary Experiment Generation Promotion

**Date**: 2026-06-28
**Status**: Draft
**Deciders**: pelikhan

---

### Context

The `gh aw experiments` feature enables A/B testing of workflow variants by sampling from a declared variant list and accumulating run counts on a dedicated git branch. However, the feature had no built-in lifecycle mechanism: once enough data was collected to identify a winning variant, there was no supported way to close out the generation, archive the results, and advance the variant population for the next round of testing. Teams were left to manually edit workflow frontmatter and reconstruct history from scattered branch data, making iterative experiment-driven improvement error-prone and hard to audit.

### Decision

We will add a `gh aw experiments darwin <workflow> <experiment>` subcommand that closes the generational loop within the existing experiments feature. Darwin mode reads the accumulated variant counts from the experiment's state branch, ranks variants by observed count (preserving declaration order for ties), promotes the selected winner to the control slot (first position in the variant list), archives the full generation snapshot as JSON under `.github/experiments/archive/`, and optionally rewrites the workflow frontmatter in place via `--apply`. The command stays inside the existing `experiments:` feature hierarchy rather than introducing a parallel optimizer path.

### Alternatives Considered

#### Alternative 1: Separate `gh aw optimize` Command

A standalone optimizer command outside the experiments hierarchy could have been introduced (e.g., `gh aw optimize`). This would give full design freedom for the optimizer interface and data model. It was not chosen because it would duplicate state management logic (reading the experiments branch, parsing state.json), break the conceptual continuity of "experiments are lifecycle-managed via the experiments subcommand tree," and require users to learn a separate tool for what is fundamentally the next step in the experiments workflow.

#### Alternative 2: Automated CI-Driven Generation Promotion

Variant promotion could be automated at the end of each experiment cycle via a GitHub Actions workflow — no CLI interaction required. This was not chosen because it removes human review of the winner selection before promotion, makes it harder to override the promoted variant or specify a custom next-generation population, and eliminates the dry-run preview (`--apply` opt-in) that lets operators confirm the plan before mutating workflow files. Human-in-the-loop control over evolutionary selection was judged important during the experimental phase of the feature.

### Consequences

#### Positive
- Closes the generation lifecycle loop: variant ranking, archiving, and promotion are now a single auditable step
- Reuses the existing experiment state branch and `state.json` history with no new storage format
- The `--apply` flag makes the command a safe dry-run by default — operators can preview the plan before any files are mutated
- JSON output (`--json`) enables machine-readable pipelines and downstream automation
- Archives provide a timestamped, queryable history of each generation and its promotion decision

#### Negative
- Darwin mode only rewrites the `variants:` list in workflow frontmatter — it does not validate that the workflow prompt already handles any new variant names introduced via `--variant`; mismatches produce silent runtime drift
- The archive is written to `.github/experiments/archive/` inside the repository, which grows unbounded without a separate pruning strategy
- Tie-breaking in variant ranking relies on declaration order, which may not reflect statistical significance; no significance threshold is enforced before promotion

#### Neutral
- The `darwin` subcommand is registered as a hidden sibling of `analyze` and `list` within the experiments command group, following the established pattern for hidden experimental subcommands
- Archive filenames use UTC timestamps, which means multiple Darwin runs within the same second could collide — acceptable for the current usage pattern but worth noting
- The implementation reuses existing internal helpers (`fetchLocalExperimentDetails`, `computeExperimentAnalysis`, `parser.UpdateWorkflowFrontmatter`) with no new inter-package dependencies

---

*ADR created by [adr-writer agent]. Review and finalize before changing status from Draft to Accepted.*
