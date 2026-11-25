# Policy for the gate end-to-end test on kind.
# The gate evaluates data.agentgate.decision and denies when it is undefined.
package agentgate

decision := {"outcome": "allow", "reason": "echo is allowed"} if input.tool == "echo"

decision := {"outcome": "deny", "reason": "delete_thing on prod is denied"} if {
	input.tool == "delete_thing"
	input.arguments.id == "prod"
}

decision := {"outcome": "approve", "reason": "delete_thing needs approval"} if {
	input.tool == "delete_thing"
	not input.arguments.id == "prod"
}

decision := {"outcome": "deny", "reason": "tool is not allowed"} if {
	not input.tool in {"echo", "delete_thing"}
}
