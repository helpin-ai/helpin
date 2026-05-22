-- Migration: cleanup_orphaned_invitation_team_preassignments
--
-- Background: invitation_team_preassignments rows were never deleted when
-- their invitation was accepted or revoked. Over time these orphans inflated
-- "pending" counts in the team members dialog and rendered as empty rows
-- showing "Pending" in place of name and email.
--
-- The application code has been updated to clean these up on accept/revoke
-- going forward. This migration backfills the existing leak.

DELETE FROM invitation_team_preassignments
WHERE invitation_id IN (
    SELECT id FROM workspace_invitations WHERE status != 'pending'
);
