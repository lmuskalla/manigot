package resolve

// This file holds the Spec for each safecode host command the TUI shells out
// to. They are functions rather than package-level variables so a caller cannot
// mutate the shared candidate list by accident.
//
// sc/sc-job/sc-done/sc-delete/sc-tui are the only supported install names.
// Earlier naming (safecode, safecode-job, safecode-done, and the standalone
// new-job / finish-job commands) is not carried forward as a legacy fallback
// — a stale install must be re-run through `make install` rather than
// resolved around.

// Safecode describes the safecode launcher itself (scripts/run.sh).
func Safecode() Spec {
	return Spec{
		Label:  "sc",
		EnvVar: EnvSafecode,
		Names:  []string{"sc"},
		Script: "scripts/run.sh",
	}
}

// Job describes the command that scaffolds a new job (scripts/new-job.sh).
func Job() Spec {
	return Spec{
		Label:  "sc-job",
		EnvVar: EnvJob,
		Names:  []string{"sc-job"},
		Script: "scripts/new-job.sh",
	}
}

// Done describes the command that finishes a job (scripts/finish-job.sh). Used
// by hostcmd.DoneCommand for the job detail view's "mark done" (capital D) key.
func Done() Spec {
	return Spec{
		Label:  "sc-done",
		EnvVar: EnvDone,
		Names:  []string{"sc-done"},
		Script: "scripts/finish-job.sh",
	}
}

// Delete describes the command that permanently deletes a job
// (scripts/delete-job.sh). Used by hostcmd.DeleteCommand for the job detail
// view's "delete" (del) key.
func Delete() Spec {
	return Spec{
		Label:  "sc-delete",
		EnvVar: EnvDelete,
		Names:  []string{"sc-delete"},
		Script: "scripts/delete-job.sh",
	}
}
