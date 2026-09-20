# Alias metadata picker search

The alias label and annotation pickers support Vim-style `/` search. A case-insensitive query filters the current picker options by their `key=value` text without changing the resource-scoped source set established in ADR 0022; `Enter` leaves search input active as a picker filter, while `Esc` clears it and returns to picker navigation. This keeps large observed-metadata lists keyboard-operable without adding a separate search view.
