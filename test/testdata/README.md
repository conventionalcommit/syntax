# Public commit corpus

real_commits.json pins 31 cases from eight public repositories. Twenty-four
contain an exact subject, and seven contain a complete message returned by the
public commit API. Selection is explicit in each record. Every record includes
the full commit identifier and a link to the source commit.

| Repository | Cases | Complete messages | Accepted by this grammar |
| --- | --- | --- | --- |
| [Angular](https://github.com/angular/angular) | 5 | 2 | 5 |
| [Vue](https://github.com/vuejs/core) | 6 | 3 | 6 |
| [Vite](https://github.com/vitejs/vite) | 3 | 0 | 3 |
| [React](https://github.com/facebook/react) | 3 | 0 | 0 |
| [Go](https://github.com/golang/go) | 3 | 0 | 3 |
| [Linux](https://github.com/torvalds/linux) | 3 | 0 | 2 |
| [Rust](https://github.com/rust-lang/rust) | 4 | 1 | 0 |
| [Gitmoji](https://github.com/carloscuesta/gitmoji) | 4 | 1 | 0 |

The corpus includes custom types, scoped headers, release subjects, dependency
updates, pull-request references, ordinary bodies, hash footers, repeated
footers, merge summaries, bracketed subjects, and emoji prefixes. Nineteen cases
are accepted and twelve are rejected by the documented package grammar.
Rejection does not mean a message is invalid for Git or its originating project.
Emoji prefixes and emoji in descriptions are separate cases: the package
accepts emoji content, while its type grammar requires the documented character
classes.

Expected fields, byte spans, and rejection offsets were reviewed against the
contract. They are stored independently of parser results. No upstream parser
source or tests are included. Messages are short quoted repository metadata;
the source links identify their origins.

The tests and fuzz seeds load the checked-in JSON. They never fetch messages
or contact a remote service. To review or add a case, inspect its source commit,
choose the subject or the complete message, preserve that text exactly, and
record expectations before running the parser. A shortened subject must not be
labelled as a complete message.
