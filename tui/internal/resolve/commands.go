package resolve

// This file holds the Spec for each manigot host command the TUI shells out
// to. They are functions rather than package-level variables so a caller cannot
// mutate the shared candidate list by accident.
//
// mg/mg-job/mg-done/mg-delete/mg-tui/mg-jdi are the only supported install
// names. Earlier naming (manigot, manigot-job, manigot-done, and the
// standalone new-job / finish-job commands) is not carried forward as a
// legacy fallback — a stale install must be re-run through `make install`
// rather than resolved around.

// Manigot describes the manigot launcher itself (scripts/run.sh).
func Manigot() Spec {
	return Spec{
		Label:  "mg",
		EnvVar: EnvManigot,
		Names:  []string{"mg"},
		Script: "scripts/run.sh",
	}
}

// Job describes the command that scaffolds a new job (scripts/new-job.sh).
func Job() Spec {
	return Spec{
		Label:  "mg-job",
		EnvVar: EnvJob,
		Names:  []string{"mg-job"},
		Script: "scripts/new-job.sh",
	}
}

// Done describes the command that finishes a job (scripts/finish-job.sh). Used
// by hostcmd.DoneCommand for the job detail view's "mark done" (capital D) key.
func Done() Spec {
	return Spec{
		Label:  "mg-done",
		EnvVar: EnvDone,
		Names:  []string{"mg-done"},
		Script: "scripts/finish-job.sh",
	}
}

// Delete describes the command that permanently deletes a job
// (scripts/delete-job.sh). Used by hostcmd.DeleteCommand for the job detail
// view's "delete" (del) key.
func Delete() Spec {
	return Spec{
		Label:  "mg-delete",
		EnvVar: EnvDelete,
		Names:  []string{"mg-delete"},
		Script: "scripts/delete-job.sh",
	}
}

// Jdi describes the autonomous-mode command (scripts/jdi.sh). Used by
// launch.Jdi for the job detail view's "run mg-jdi" (capital J) key.
func Jdi() Spec {
	return Spec{
		Label:  "mg-jdi",
		EnvVar: EnvJdi,
		Names:  []string{"mg-jdi"},
		Script: "scripts/jdi.sh",
	}
}
