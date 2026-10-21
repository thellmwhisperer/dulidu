# Prompts

Each P01-P18 template is one prompt variant: the poc role text, then the context its builder added.
`Render(step, data)` fills it; a key the data lacks fails with its name.

Keys: `Input` (brief and attachments), `Research`, `Personas`, `Functionalities`, `MustFunctionalities`,
`Flows`, `Constraints` (text the caller formats), `Epic` (a `pipeline.EpicOutline`), `Epics` and `Stories`
(counts, 0 for no target), `Screens` (text) and `Screen` (a `pipeline.Screen`, its name and purpose).

All templates ask for plain text. P01, P02, P08 and P14 are retained variants whose original structured answer examples have also been replaced with text. The run folder still stores its typed files as JSON.

`triage` is new, not from the poc: the triage step's prompt, with `Context` (contexto.md) as its only key.

## Source and removals

Sources are in the reference snapshot c10131b, `poc/src/pipeline/`. Removed sentences are described, not quoted.

| ID | Source | Template | Removed or changed |
|---|---|---|---|
| P01 | `triage.ts:6-18, 54-56` | `p01-triage-classifier` | A named company in the complex example, now an enterprise dashboard; answer format changed from JSON. |
| P02 | `steps/step0-research.ts:6-15, 48-54` | `p02-research-full` | Answer format changed from JSON. |
| P03 | `steps/step0-research.ts:21, 56-62` | `p03-research-business` | The answer format and research instructions changed; see [research usage](../../../README.md#commands). |
| P04 | `steps/step0-research.ts:26, 56-62` | `p04-research-users` | Same. |
| P05 | `steps/step0-research.ts:31, 56-62` | `p05-research-market` | Same. |
| P06 | `steps/step0-research.ts:36, 56-62` | `p06-research-technical` | Same. |
| P07 | `steps/step0-research.ts:41, 56-62` | `p07-research-project` | Same. |
| P08 | `steps/step1-analysis.ts:6-13, 46-60` | `p08-analysis-full` | The five fixed research labels: `Research` comes formatted; answer format changed from JSON. |
| P09 | `steps/step1-analysis.ts:19, 62-76` | [p09-analysis-personas](p09-analysis-personas.md.tmpl) | Same: `Research` is the five dimensions as named lines, `Business:` to `Project:`; answer format changed from JSON. |
| P10 | `steps/step1-analysis.ts:24, 62-76` | [p10-analysis-functionalities](p10-analysis-functionalities.md.tmpl) | The same context as P09: the brief and the five research dimensions as named paragraphs, no list asked before; answer format changed from JSON. |
| P11 | `steps/step1-analysis.ts:29, 62-76` | [p11-analysis-flows](p11-analysis-flows.md.tmpl) | The same context as P09; answer format changed from JSON. |
| P12 | `steps/step1-analysis.ts:34, 62-76` | [p12-analysis-constraints](p12-analysis-constraints.md.tmpl) | The same context as P09; the mobile-platform example constraint was removed and replaced; answer format changed from JSON. |
| P13 | `steps/step1-analysis.ts:39, 62-76` | [p13-analysis-questions](p13-analysis-questions.md.tmpl) | The same context as P09; answer format changed from JSON. |
| P14 | `steps/step2-epics.ts:5-16, 76-96` | `p14-epics-full` | The issue-tracker brand the epics were likened to; answer format changed from JSON. |
| P15 | `steps/step2-epics.ts:18-34, 98-123` | [p15-epics-outlines](p15-epics-outlines.md.tmpl) | Product-specific BAD/GOOD epic names (line 23-24), now generic; "for a solo dev iterating fast" (26); "a solo developer ... shippable app" (106), now "a shippable product". The answer is now plain-text epic blocks with `Depends on:` lines; see the [run behavior](../../../README.md#commands) for cap handling. |
| P16 | `steps/step2-epics.ts:36-65, 125-166` | [p16-epics-stories](p16-epics-stories.md.tmpl) | The product's model-API timeout example (45), now a generic one; six exclusions tied to that project: monitoring, docs, versioning, remote caching and CI, responsive layout, navigation persistence (48-53); tool names in the setup-story rule (54); the image-resize title example (60); the 50-constraint cut (160), left to the caller. The answer is now plain-text story blocks with `Functionalities:` ids, `### Acceptance` lists and `### First test` prose, plus an optional `Type: feature | fix` line and `### Use cases`, `### Invariants`, `### Non-goals` and `### Split triggers` lists; see the [run behavior](../../../README.md#commands) for cap handling. |
| P17 | none: the design step's first phase, new | [p17-design-screens](p17-design-screens.md.tmpl) | Not from the snapshot: it asks for the screens of the analysis' personas, functionalities and flows, one `## <name>` block each with its purpose as prose. |
| P18 | the prototype's per-screen script, its `systemPrompt`, outside the snapshot | [p18-design-screen](p18-design-screen.md.tmpl) | Kept: the skeletons designer role, the ASCII wireframe 30+ lines tall, proportional, in box-drawing chars, the components, navigation, states and the colour-blind sentence. Removed: the product's brand, colours, touch sizes and names. The answer is a `## <name>` block with `### Wireframe` in a fenced block, `### Components` and `### States` (`- <state>: <description>`) lists, and `### Navigation` and `### Accessibility` prose, in place of the id and fields the script asked for. |
