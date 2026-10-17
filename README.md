# dulidu

One binary in the PATH. Run `init`, `doctor`, `run`, `runs`, and `release` inside a Git repository.

```sh
curl -fsSL https://raw.githubusercontent.com/thellmwhisperer/dulidu/main/install.sh | sh
dulidu init     # writes dulidu.yaml and ignores .dulidu/
dulidu doctor   # checks go, git, the config layers and video tools
dulidu doctor --agent # asks the configured agent to say OK and reports elapsed time
dulidu run brief.md --attach notes.pdf --attach shot.png --attach clip.mp4 # writes contexto.md, triage.json, research.json, analysis.json, epics.json, then spec.md, tickets/, plan.json and brief-diseno.md
dulidu run --resume <label> --answers answers.md # answers triage's questions and goes on from research to the export
dulidu runs     # lists .dulidu/runs/ newest first: label, status, last step, started
dulidu update   # updates an installed release to the latest release
dulidu release vX.Y.Z # builds and publishes a tagged release
```

Configuration is layered: built-in defaults, `~/.config/dulidu/config.yaml`, then the repository's `dulidu.yaml`.
`dulidu init` writes a template with every required project and step key. Each step has its own agent CLI, model and effort, with suggested defaults. Top-level harness, model and effort configure `doctor --agent`. Every required key must be present across the layers, even when empty; unknown keys are rejected. Errors name the key as `config incomplete: <key>`. The agent uses its installed CLI and existing authentication.

`run` writes a new `.dulidu/runs/<label>/` with the brief and its attachments in `input/` and `contexto.md`: an index, then one section per file. It reads pdf, docx, md, txt and json as text; png, jpg, webp and heic as images. Videos in mp4, mov or webm format, up to 30 minutes long, produce an initial frame, further frames every `video_frame_seconds` seconds (10 when empty), and one contact sheet in `input/frames/<video>/`. The `steps.input` agent describes each image and each video contact sheet once. A video with audio also has a transcript from `whisper-cli`, using local weights from `WHISPER_MODEL` or `models/ggml-base.en.bin` by default. Any other extension stops the run before any agent call.

After packaging, `steps.triage` reads `contexto.md` once and writes `triage.json` with project complexity and type, epic and story caps, research dimensions, available context and missing questions. Positive `max_epics` and `max_stories_per_epic` settings limit those caps; `report.json` records each clamp in the triage step's `note`. An invalid answer fails at triage without creating `triage.json`. When questions are missing, `run` writes them to `questions.json`, prints them as that file has them, sets the run's status to `waiting_answers` and exits 0. `dulidu run --resume <label> --answers answers.md` appends the file under `## Answers` at the end of that run's `contexto.md`, sets it `running` and goes on to research in the same run folder, without running input or triage again. Resuming a run in any other status fails with `run <label> is <status>, nothing to resume`.

Without missing questions, or once they are answered, `steps.research` is called once per research dimension triage switched on, in triage's order, with that dimension's prompt and `contexto.md`. Each answer must be `{text, sources: [{url, title, supports}]}` with text and at least one source, every field filled; sources are recorded, not fetched or checked. `research.json` holds exactly those dimensions, `dimensions: []` when triage switched none on, and `report.json` records the research step's harness, model, effort and dimension names. The first invalid answer stops the run with `failed at research: <dimension>: <reason>`, such as `no sources`, without asking the rest or writing `research.json`.

After research, `steps.analysis` writes `analysis.json` with its five lists; the first invalid list stops the run with `failed at analysis: <list>`. Then `steps.epics` writes `epics.json`, outlines first and then each epic's stories, on the same agent (`steps.stories` is not used yet); a failed check stops the run with `failed at epics: <check>`. Last, the export writes `spec.md`, `tickets/`, `plan.json` and `brief-diseno.md` by code alone. A failed step writes nothing after it. `report.json` records every step; agent steps include their effective harness, model and effort, while export records its status without agent settings.

PDF extraction requires `pdftotext` (`brew install poppler`, `apt install poppler-utils`) and nonempty text; image dimensions require `ffprobe` (`brew install ffmpeg`, `apt install ffmpeg`) and positive width and height. Missing `ffmpeg` or `ffprobe` leaves a note in the video section; missing `whisper-cli` leaves a transcript note when the video has audio. Ordinary `doctor` checks these video tools and gives install hints; `doctor --agent` checks the agent without requiring them. An empty `steps.input.model` delegates model selection to the agent.

`update` leaves development builds untouched. Before building, `release` checks that an existing tag on `origin` points to the current commit. Publication uses `origin` even if `GH_REPO` is set.

Develop with `make test lint`; `make dist TAG=vX.Y.Z` builds assets without publishing, and `make release TAG=vX.Y.Z` publishes them. macOS and Linux only.
