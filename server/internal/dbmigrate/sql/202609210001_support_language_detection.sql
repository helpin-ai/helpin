-- Keep language classification independent from full-message translation.
ALTER TABLE jev_decision_attempts DROP CONSTRAINT IF EXISTS jev_decision_attempts_feature_check;
ALTER TABLE jev_decision_attempts ADD CONSTRAINT jev_decision_attempts_feature_check CHECK (feature IN ('meeting_routing','coverage_classification','coverage_topic_matching','automation_condition','answer_evidence','translation_review','language_detection'));
ALTER TABLE support_translations DROP CONSTRAINT IF EXISTS support_translations_purpose_check;
ALTER TABLE support_translations ADD CONSTRAINT support_translations_purpose_check CHECK (purpose IN ('message_display','outgoing_reply','language_detection'));
ALTER TABLE support_translations DROP CONSTRAINT IF EXISTS support_translations_check;
ALTER TABLE support_translations ADD CONSTRAINT support_translations_check CHECK ((purpose IN ('message_display','language_detection') AND source_message_id IS NOT NULL AND sent_message_id IS NULL) OR (purpose='outgoing_reply' AND source_message_id IS NULL AND created_by_user_id IS NOT NULL));
