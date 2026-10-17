# Add a README paragraph for private-clone installs and label the curl line as the public-repository form

Type: feature

## Kind

implement

## Goal

As a reader who has a clone of the private Dulidu repository I want a README paragraph telling me to run the install.sh in my clone with DULIDU_PREFIX set to my chosen directory so that I never run the curl command that returns 404 while the repo is private, and as a public-repo installer I still see the curl line labelled for me.

## Non-goals

- No installer code, no new command, no Windows, no signing.
- No change outside README.md and one new test file under scripts/.

## Invariants

- No installer code changes
- No new command
- No Windows support
- No signing
- Must keep the existing curl install line in the README, labelled as the form for a public repository
- The curl line stays character for character
- Private-clone readers must run the install.sh already in their clone, not a downloaded one
- The paragraph must show setting DULIDU_PREFIX to the directory the reader chooses, with a concrete example command
- A test file under scripts/ is in scope. It is the acceptance check. It is not installer code.

## Behavioral contract / States

- Given the README install section, when a reader scans it, then one new paragraph says that readers who already have a clone of the private repository should run the install.sh that is in the clone, and states that the curl line returns 404 while the repository is private.
- Given that paragraph, when a reader looks for how to pick the install directory, then it shows DULIDU_PREFIX set to the directory the reader chooses, with a concrete example command run from the clone (for example: DULIDU_PREFIX=$HOME/.local sh ./install.sh), and the example uses the local ./install.sh path, not a download.
- Given the README, when a reader looks at the existing line `curl -fsSL https://raw.githubusercontent.com/thellmwhisperer/dulidu/main/install.sh | sh`, then that line is still present, unchanged character for character, and is labelled as the form for a public repository.

## Scope surface

- README.md
- scripts/*_test.go
- F1 [must] Local install instructions for private-repo clones
- F2 [must] DULIDU_PREFIX install directory guidance
- F3 [must] Keep curl install line labelled for public repositories

## Acceptance

- [ ] A Go test under scripts/ reads README.md and fails on current main. It passes when: (a) the exact curl line is present and a nearby label contains the words "public repository"; (b) the install section contains "clone", "private", and a line matching `DULIDU_PREFIX=\S+ .*install\.sh` that does not use curl. The test does not run git diff.

## Split triggers

- The acceptance needs more than one first test to go red: split the ticket.

## Dependencies / Parent

- Parent: E1 README install paths for private clone and public repository.
- Story: US1.
- Depends on: none.

## Evidence (fix / ops)

- Not applicable: a feature.
- Cut from the raw export: the export also required a diff of only README.md, which cannot pass while the acceptance adds a test file. That clause is removed here. The raw export is kept beside this file.

## Validation and rollback

Revert the PR.
