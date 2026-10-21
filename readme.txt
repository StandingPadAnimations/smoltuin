smoltuin
--------
smoltuin is a history manager for Bash. It allows
for searching previously ran commands, with filtering
by either the working directory or exit status.

This started a personal fork of stinkpot (https://tangled.org/oppi.li/stinkpot/)
that removed support for shells other than Bash, and add support
for importing Atuin history, filtering by working directory,
and filtering by whether a command exited with an error or not.
Given this became a hard fork, I decided to rename it to
smoltuin, which is based on Atuin (https://atuin.sh) but
with the word "smol", as this aims to be a smaller Atuin,
with one purpose only: store shell history, and provide
fuzzy searching and filtering. No sync, no AI, nothing other
than shell history. This allows for a much smaller codebase
(under 1000 lines of code), and a smaller binary (8 MB vs
41 MB for Atuin 18.23).

While Bash is the only natively supported shell (as in we
have a configuration for it out of the box), the program
itself can be used in any shell. Hell, you can even pipe
it with other programs in the command line if you wish.

Commits starting from c84b6c6451e9a750f944ac2d3e7c6a54758d314e
are licensed under the GNU General Public License v3.0 or Later.
Commits before said commit are licensed under the MIT License.

Develop
-------

To build:

    go build -ldflags "-s -w" -o smoltuin .

Usage
-----

Just call eval the init script in your .bashrc:

    eval "$(smoltuin init)"

If migrating from Atuin, import your history:

    smoltuin import

and then hit ctrl+r at your shell to trigger reverse search.
Keybinds for filtering are located at the bottom:

    Up/Down     - Move
    Enter       - Confirm selection
    Tab         - Toggle between commands executed in the working
                  directory and everywhere
    Shift + Tab - Sort by either time or fuzzy matching score
    Ctrl + A    - Hide commands that exited with a status other
                  than 0
