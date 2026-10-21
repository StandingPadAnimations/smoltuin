stinkpot
--------
                             _
                         _  / \  _
                        (_\/\_/\/_)
                          /\/ \/\
                          \/\_/\/
                           \/_\/
                           "   "

stinkpot is an sqlite backed shell-history searcher for
Bash.

This is a personal fork that removes support for shells
other than Bash, and adds support for importing Atuin
history. In addition, it also adds filtering by working
directory (with toggling) and by whether a command exited
with an error code or not. The original project is availible
on Tangled at oppi.li/stinkpot (https://tangled.org/oppi.li/stinkpot/).
Commits starting from c84b6c6451e9a750f944ac2d3e7c6a54758d314e
are licensed under the GNU General Public License v3.0 or Later.

stinkpot is a much tinier Atuin (https://atuin.sh). I have
been using Atuin for a while, but I don't use most of its
features: the sync server, Atuin AI, dotfiles manager,
script manager or the KV store. while sync is interesting,
my machines have very different setups and I tend to run
very different commands on them.

My only usecase for Atuin was the session agnostic history
management, and the searcher TUI. stinkpot provides these
while being about 630 lines of go.

stinkpot is a tiny turtle species apparently, hence the
name.

Develop
-------

To build:

    go build -ldflags "-s -w" -o stinkpot .

Usage
-----

Just call eval the init script in your .bashrc:

    eval "$(stinkpot init)"

If migrating from Atuin, import your history:

    stinkpot import 

and then hit ctrl+r at your shell to trigger reverse search.
Keybinds for filtering are located at the bottom:

    Up/Down     - Move
    Enter       - Confirm selection
    Tab         - Toggle between commands executed in the working
                  directory and everywhere
    Shift + Tab - Sort by either time or fuzzy matching score
    Ctrl + A    - Hide commands that exited with a status other
                  than 0
