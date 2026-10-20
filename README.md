# dulidu

dulidu turns a product brief and what comes with it (pdfs, images, videos, notes) into a spec, tickets in the
ten-point contract, a `plan.json` and a design brief, one agent call per step, in a run folder inside your Git
repository. Each ticket is ready to hand on.

## Install

For a public repository:

```sh
curl -fsSL https://raw.githubusercontent.com/thellmwhisperer/dulidu/main/install.sh | sh
```

While the repository is private, that curl line returns 404. If you already have a clone of the private repository,
run the `install.sh` that is in the clone, with `DULIDU_PREFIX` set to the directory you choose for the binary:

```sh
DULIDU_PREFIX=$HOME/.local/bin sh ./install.sh
```

From a clone, `go build -o dulidu ./cmd/dulidu` and put the binary in your PATH. macOS and Linux only.
A run with a pdf needs `pdftotext` and one with images or videos `ffprobe`; `dulidu doctor` says what is missing.

## First run

The example is Clay Corner, the recorded run in `internal/pipeline/testdata/run-example/`: a brief for a pottery
studio's booking tool, `brief.md`, with a pdf, `one.pdf`, and an image, `one.png`. Copy those three into a new
Git repository and run, in it:

```console
$ dulidu init
$ dulidu doctor
ok go: /opt/homebrew/bin/go
ok git: /private/tmp/dulidu-readme.rIMby2/work
ok config: /private/tmp/dulidu-readme.rIMby2/work/dulidu.yaml
$ dulidu run brief.md --attach one.pdf --attach one.png
/private/tmp/dulidu-readme.rIMby2/work/.dulidu/runs/20261017-071309-1746844562/contexto.md
$ dulidu status
20261017-071309-1746844562 done
input done 0 min 00 s
triage passed 0 min 00 s
research passed 0 min 00 s
analysis passed 0 min 00 s
epics passed 0 min 00 s
export passed 0 min 00 s
$ ls .dulidu/runs/*/ .dulidu/runs/*/tickets/
.dulidu/runs/20261017-071309-1746844562/:
agent
agent.out
analysis.json
brief-diseno.md
contexto.md
epics.json
input
plan.json
report.json
research.json
spec.md
tickets
triage.json

.dulidu/runs/20261017-071309-1746844562/tickets/:
1.md
2.md
3.md
$ cat .dulidu/runs/*/tickets/1.md
# Book a free wheel for an hour

Type: feature

## Kind

implement

## Goal

As Ana I want to book a free wheel so that one waits for me

## Non-goals

- Anything beyond this ticket's acceptance: the plan's other tickets deliver it.

## Invariants

- The kiln fires once a week, on Friday at 6 pm
- Bookings are free: members pay a monthly fee

## Behavioral contract / States

- Given wheel 2 free at 19:00, when Ana books it, then it is hers from 19:00 to 20:00

## Scope surface

- F1 [must] Book a wheel: One hour, one member per wheel

## Acceptance

- [ ] Write a test that books a free wheel at 19:00 and expects it held until 20:00.

## Split triggers

- The acceptance needs more than one first test to go red: split the ticket.

## Dependencies / Parent

- Parent: E1 Book a wheel.
- Story: US1.
- Depends on: none.

## Evidence (fix / ops)

- Not applicable: a feature.

## Validation and rollback

Revert the PR.
```

With a real agent each step calls `claude`, so the label and timings differ. The output above is `sh scripts/readme-check.sh`'s: it runs this
block's `$` lines in a new repository holding the example, against a fresh build, with a `claude` that replays
the example's recorded answers in `answers/`, offline, and fails unless the run folder's export is
`docs/example-run/`'s files.

## What comes out

The run folder `.dulidu/runs/<label>/` keeps the inputs in `input/`, `contexto.md`, each step's answer
(`triage.json`, `research.json`, `analysis.json`, `epics.json`) and `report.json`, then the export, written by
code alone. Clay Corner's export is in [docs/example-run/](docs/example-run/):

- [`spec.md`](docs/example-run/spec.md): research with its sources, personas, functionalities, flows,
  constraints, open questions and the epics with their tickets.
- [`tickets/`](docs/example-run/tickets/): one ticket per story, `<n>.md`, in the ten-point contract.
- [`plan.json`](docs/example-run/plan.json): each ticket's issue number and the tickets it depends on.
- [`brief-diseno.md`](docs/example-run/brief-diseno.md): personas, flows, functionalities and constraints, for
  design.

## Handing a ticket on

A ticket is a file, or an issue number once it is filed. Ticket 1 above is:

```sh
.dulidu/runs/20261017-071309-1746844562/tickets/1.md
```

`plan.json` gives the order: ticket 3 waits for 1 and 2. `readme-check.sh` does not hand a ticket on.

A real one: [`private-install-brief.md`](docs/private-install-brief.md) went through `dulidu run`, and its
exported ticket, [`private-install-ticket.md`](docs/private-install-ticket.md), is:

```sh
docs/private-install-ticket.md
```

The result is [PR #43](https://github.com/thellmwhisperer/dulidu/pull/43), merged as `401a878`.

## Commands

```sh
dulidu init     # writes dulidu.yaml and ignores .dulidu/
dulidu doctor   # checks go, git, the config layers and video tools
dulidu doctor --agent # asks the configured agent to say OK and reports elapsed time
dulidu run brief.md --attach notes.pdf --attach shot.png --attach clip.mp4 # writes contexto.md, triage.json, research.json, analysis.json, epics.json, then spec.md, tickets/, plan.json and brief-diseno.md
dulidu attach <label> # pipeline box and Agent box, scroll; q detaches, t folds tools, Tab moves focus; piped, one line per step
dulidu status [label] # a run's label and status, one line per step; the newest run's without a label
dulidu runs     # lists .dulidu/runs/ newest first: label, status, last step, started
dulidu update   # updates an installed release to the latest release
dulidu release vX.Y.Z # builds and publishes a tagged release
```

Configuration is layered: built-in defaults, `~/.config/dulidu/config.yaml`, then the repository's `dulidu.yaml`.
`dulidu init` writes a template with every required project and step key. Each step has its own agent CLI, model and effort, with suggested defaults. Top-level harness, model and effort configure `doctor --agent`. Every required key must be present across the layers, even when empty; unknown keys are rejected. Errors name the key as `config incomplete: <key>`. The agent uses its installed CLI and existing authentication.

`run` writes a new `.dulidu/runs/<label>/` with the brief and its attachments in `input/` and `contexto.md`: an index, then one section per file. It reads pdf, docx, md, txt and json as text; png, jpg, webp and heic as images. Videos in mp4, mov or webm format, up to 30 minutes long, produce an initial frame, further frames every `video_frame_seconds` seconds (10 when empty), and one contact sheet in `input/frames/<video>/`. The `steps.input` agent describes each image and each video contact sheet once. A video with audio also has a transcript from `whisper-cli`, using local weights from `WHISPER_MODEL` or `models/ggml-base.en.bin` by default. Any other extension stops the run before any agent call.

After packaging, `steps.triage` reads `contexto.md` once and writes `triage.json` with the project's complexity, its epics, and the reasoning. The complexity caps the epics: simple 3, moderate 5, complex 8. A positive `max_epics` clamps them further, the clamp in the triage step's `note` in `report.json`; a positive `max_stories_per_epic` caps the stories in each epic. An agent error or an invalid answer is a failed classification: the triage passes as moderate with 5 epics before any `max_epics` clamp with its note starting `classification failed, moderate`. On a terminal, `run` runs in the background instead and attaches, the live view of `dulidu attach`: a rounded pipeline box, one row per step with its state icon, label, duration and note, and a rounded Agent box with what the agent says and does. From 100 columns the pipeline box is on the left and the Agent box on the right. Below 100 columns the pipeline box is on top. `q` detaches and the run goes on; `t` folds or unfolds tool calls while the Agent box shows; `Tab` moves the `*` between the pipeline box and the Agent box; the arrow keys, page keys, Home, End and the mouse wheel scroll. A box scrolled up stays, and follows again at its last line. Once the view ends a failed run fails the command. A failed step replaces the Agent box with a Findings box that shows the reason, above a Log box of the last agent lines.

After triage, `steps.research` is called once for each of business, users, market, technical and project, in that order, with that dimension's prompt and `contexto.md`. With Claude, research enables `WebSearch` and `WebFetch` and asks the agent to search the internet, read the pages it cites, and read no local file other than `contexto.md` in the run folder; other harnesses keep their existing tools. Each answer must be `{text, sources: [{url, title, supports}]}` with text and at least one source, every field filled; the application records sources but does not independently fetch or check them. `research.json` holds all five dimensions regardless of triage's answer, and `report.json` records the research step's harness, model, effort and dimension names, with `web: on` at the end of its note for Claude. The first invalid answer stops the run with `failed at research: <dimension>: <reason>`, such as `no sources`, without asking the rest or writing `research.json`.

After research, `steps.analysis` writes `analysis.json` with its five lists; the first invalid list stops the run with `failed at analysis: <list>`. Then `steps.epics` writes `epics.json`, outlines first and then each epic's stories, on the same agent (`steps.stories` is not used yet); a failed check stops the run with `failed at epics: <check>`. Last, the export writes `spec.md`, `tickets/`, `plan.json` and `brief-diseno.md` by code alone. A failed step writes nothing after it. `report.json` records every step; agent steps include their effective harness, model and effort, while export records its status without agent settings.

PDF extraction requires `pdftotext` (`brew install poppler`, `apt install poppler-utils`) and nonempty text; image dimensions require `ffprobe` (`brew install ffmpeg`, `apt install ffmpeg`) and positive width and height. Missing `ffmpeg` or `ffprobe` leaves a note in the video section; missing `whisper-cli` leaves a transcript note when the video has audio. Ordinary `doctor` checks these video tools and gives install hints; `doctor --agent` checks the agent without requiring them. An empty `steps.input.model` delegates model selection to the agent.

`update` leaves development builds untouched. Before building, `release` checks that an existing tag on `origin` points to the current commit. Publication uses `origin` even if `GH_REPO` is set.

Develop with `make test lint`; `sh scripts/readme-check.sh` replays the first run and `sh scripts/leak-scan.sh README.md docs/` checks the docs for paths, addresses and tokens. `make dist TAG=vX.Y.Z` builds assets without publishing, and `make release TAG=vX.Y.Z` publishes them. macOS and Linux only.
