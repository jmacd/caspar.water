# AGENTS.md — Working process for the caspar.water pair

This file defines how automated agents (and humans) manage repositories, branches,
and state in this project. Read it before making any change.

## 1. The "pair" is the unit of work

Development happens in a **pair**:

```
caspar.water/            <- outer repo (this repo): configuration, blog, deployment
└── watertown/           <- submodule: the Watertown platform source (git@github.com:jmacd/watertown.git)
```

Other submodules also exist (`noyo-blue-econ`, `opentelemetry-mqtt-sparkplug`), but
the **caspar.water + watertown** pair is where nearly all active work happens.

**Golden rule: stay inside one pair directory for the entire session.**

- All Watertown work goes through `caspar.water/watertown` (the submodule), never a
  standalone/sibling clone of `watertown` located elsewhere on disk.
- Never `cd` out of the pair to a separate checkout of the same repo. Separate
  standalone clones are *different, independent threads* and must not be touched.

### Running a second pair in parallel

To do parallel independent work, create a **second full clone** of `caspar.water`
(with its submodules) in a separate directory — a second self-contained pair. Each
pair is one thread. Because both pairs push to the same GitHub repos
(`jmacd/caspar.water`, `jmacd/watertown`), the two pairs **must use distinct branch
names** to avoid collisions on the remote.

## 2. Branch ownership: the human owns branches, the agent does not

To keep branch state unambiguous:

- The **agent never** runs `git checkout`, `git switch`, `git branch`, or `-b`, and
  never creates, renames, or deletes branches — in either repo of the pair.
- The agent works only on **whatever branch is currently checked out** in each repo.
- When new work needs a fresh branch, the agent **says so and stops** (e.g.,
  "this needs a fresh branch; suggested name `jmacd/<topic>`"). The human runs their
  own checkout ritual.
- The agent may **suggest** branch names but must not act on them.

Branch names are **descriptive** (`jmacd/<topic>`), not numbered.

## 3. Two independent lifecycles

The two repos in the pair move at different cadences and do **not** have to share a
branch name (they may share one when a single change spans both).

### watertown (fast — most PRs; each merge produces a built image)

After a watertown PR merges on GitHub, the human resets **inside the submodule**:

```bash
cd watertown
git checkout main && git pull
git checkout -b jmacd/<topic>
```

### caspar.water (slow — configuration only)

caspar.water holds mainly configuration and blog content. It usually does **not**
need a commit; it is committed only when:

- blog / site content changes, or
- the watertown submodule pointer should move (see §4).

Its reset ritual, when needed, is the same:

```bash
git checkout main && git pull
git checkout -b jmacd/<topic>
```

Often you simply **stay on `main`** in caspar.water and branch only when there is a
real config/blog/pointer change.

## 4. Submodule pointer bumps are deliberate

The agent **never** bumps the `watertown` submodule pointer silently. Moving the
pointer to a merged watertown `main` is an explicit, human-approved action, made as a
dedicated commit (e.g., `watertown: bump to #NNN`), only when that watertown state
should actually deploy.

## 5. Start-of-turn state report

At the start of any turn that will touch git, the agent first **reports state**:
which branch each repo of the pair is on, and whether each is behind its
`origin/main`. The human always sees where things stand before anything is modified.

## 6. History hygiene

- **Never** `git amend` or `git rebase`. Reconcile diverged branches by **merging**
  from origin (`git merge origin/main`), never by rewriting history.
- The **human merges PRs**. The agent opens/pushes branches and leaves merging to the
  human.

## 7. Quick checklist for agents

- [ ] Am I inside the intended pair directory (not a sibling clone)? 
- [ ] Did I report current branch state before touching git?
- [ ] Am I working on the already-checked-out branch (not creating one)?
- [ ] Did I avoid amend/rebase and any silent submodule-pointer bump?
- [ ] Am I leaving the PR merge to the human?
