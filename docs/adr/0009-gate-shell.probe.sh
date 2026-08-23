#!/bin/sh
# posse gate shell — reference for ADR 0009 (verified 2026-08-18 on grok 1.0.5, claude, codex 0.147);
# the launcher renders this from Go. Stands in for the login shell a runtime re-execs.
G=__GATES_BIN__   # rendered: RHQ_HOME/state/gates/<persona>/bin
REAL=__REAL__
LOG=__GATES_DIR__/shell.log
PRE="case \"\$PATH:\" in \"$G\":*) ;; *) PATH=\"$G:\$PATH\";; esac; export PATH; "
# The guard asserts the gates dir is FIRST, not merely present: the typed line
# already puts it on PATH, so path_helper (via /etc/zprofile, which runs before
# this -c string) demotes it below /usr/bin instead of dropping it (ADR 0009 §1).
# Walk argv like the shell does: leading -x/+x words are options (-o/-O/+o/+O
# and --rcfile/--init-file consume a value; '--' ends them). If a -c was
# seen, the first operand is the command string: prefix it. If the operand
# after that (argv0) is '--', the next one is grok's user-command slot: prefix
# it too, so the guard runs after the snapshot replay. Everything else passes.
n=$#; i=0; st=opts; cflag=0
while [ $i -lt $n ]; do
  a=$1; shift; i=$((i+1))
  case $st in
    opts)
      case "$a" in
        --) st=str ;;
        -o|+o|-O|+O|--rcfile|--init-file) st=optval ;;
        -[!-]*) case "${a#-}" in *[!a-zA-Z]*) ;; *c*) cflag=1;; esac ;;
        --*|+*) ;;
        *) if [ $cflag -eq 1 ]; then a="$PRE$a"; st=argv0; else st=done; fi ;;
      esac ;;
    optval) st=opts ;;
    str)  if [ $cflag -eq 1 ]; then a="$PRE$a"; st=argv0; else st=done; fi ;;
    argv0) if [ "$a" = "--" ]; then st=usercmd; else st=done; fi ;;
    usercmd) a="case \"\$PATH:\" in \"$G\":*) ;; *) echo \"\$(date -u +%Y-%m-%dT%H:%M:%SZ) gates dir not first in replayed PATH; re-prepended (path_helper/rc reorder?)\" >> '$LOG' 2>/dev/null;; esac; $PRE$a"; st=done ;;
    done) ;;
  esac
  set -- "$@" "$a"
done
exec "$REAL" "$@"
