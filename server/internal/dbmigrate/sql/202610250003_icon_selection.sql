-- Allow icon suggestions through the existing bounded Jev admission and audit.
ALTER TABLE jev_decision_attempts DROP CONSTRAINT IF EXISTS jev_decision_attempts_feature_check;
ALTER TABLE jev_decision_attempts ADD CONSTRAINT jev_decision_attempts_feature_check CHECK (feature IN ('meeting_routing','coverage_classification','coverage_topic_matching','automation_condition','answer_evidence','translation_review','language_detection','icon_selection'));
