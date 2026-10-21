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
bash.

This is a personal fork that removes support for shells
other than Bash, and adds support for importing Atuin
history. In addition, it also adds filtering by working
directory (with toggling) and by whether a command exited
with an error code or not. The original project is availible
on Tangled at oppi.li/stinkpot (https://tangled.org/oppi.li/stinkpot/).
Commits starting from c84b6c6451e9a750f944ac2d3e7c6a54758d314e
are licensed under the GNU General Public License v3.0 or Later.

stinkpot is a much tinier atuin (https://atuin.sh). i have
been using atuin for a while, but i don't use most of its
features: the sync server, atuin AI, dotfiles manager,
script manager or the KV store. while sync is interesting,
my machines have very different setups and i tend to run
very different commands on them.

my only usecase for atuin was the session agnostic history
management, and the searcher TUI. stinkpot provides these
while being about 400 lines of go.

stinkpot is a tiny turtle species apparently, hence the
name.

develop
-------

to build:

    go build -ldflags "-s -w" -o stinkpot .

usage
-----

if you use home-manager for nixos, stinkpot has a
home-manager module:

    home-manager.users.<name> = {
      imports = [
        ./home.nix
        inputs.stinkpot.homeManagerModules.default
      ];
    };


if not, just call eval the init script in your .bashrc:

    eval "$(stinkpot init)"

or .config/fish/config.fish:

	stinkpot init fish | source

start by importing your existing bash history into stinkpot:

    stinkpot import 
  
and then hit ctrl+r at your shell to trigger reverse search:

    > search history...
     1s vim flake.nix
     3m redis-server
     3m jj l
    35m jj show m
    36m jj git fetch
    39m cd
    40m vim readme.txt
      788 matches · ↑/↓ move · enter accept · esc cancel


hit tab/enter to accept the selection, hit enter again to
execute. the history database is stored in
~/.local/share/stinkpot. if upgrades break the DB, just
delete it and run the import command.
