-- TeachersLounge — Teacher Accounts Schema
-- Adds teacher profiles, classes, student rosters, and material assignments.

BEGIN;

-- ============================================================
-- TEACHER PROFILES
-- A teacher_profile record marks a user as a teacher.
-- Any standard user can become a teacher by creating a profile.
-- ============================================================
CREATE TABLE teacher_profiles (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    school_name TEXT NOT NULL DEFAULT '',
    bio         TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- TEACHER CLASSES
-- ============================================================
CREATE TABLE teacher_classes (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    teacher_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    subject     TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_teacher_classes_teacher ON teacher_classes(teacher_id);

-- ============================================================
-- CLASS ENROLLMENTS (student roster)
-- ============================================================
CREATE TABLE class_enrollments (
    class_id    UUID NOT NULL REFERENCES teacher_classes(id) ON DELETE CASCADE,
    student_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (class_id, student_id)
);

CREATE INDEX idx_enrollments_student ON class_enrollments(student_id);

-- ============================================================
-- CLASS MATERIAL ASSIGNMENTS
-- ============================================================
CREATE TABLE class_material_assignments (
    class_id    UUID NOT NULL REFERENCES teacher_classes(id) ON DELETE CASCADE,
    material_id UUID NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    due_date    TIMESTAMPTZ,
    PRIMARY KEY (class_id, material_id)
);

-- ============================================================
-- ROW LEVEL SECURITY
-- Services bypass via BYPASSRLS role; these policies are
-- an additional safety layer for direct DB access.
-- ============================================================
ALTER TABLE teacher_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE teacher_classes ENABLE ROW LEVEL SECURITY;
ALTER TABLE class_enrollments ENABLE ROW LEVEL SECURITY;
ALTER TABLE class_material_assignments ENABLE ROW LEVEL SECURITY;

-- Teachers see only their own profile
CREATE POLICY teacher_isolation ON teacher_profiles
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

-- Teachers see only their own classes
CREATE POLICY teacher_isolation ON teacher_classes
    USING (teacher_id = current_setting('app.current_user_id', true)::uuid);

-- Enrollments: teachers see their class's enrollments; students see their own
CREATE POLICY teacher_isolation ON class_enrollments
    USING (
        student_id = current_setting('app.current_user_id', true)::uuid
        OR class_id IN (
            SELECT id FROM teacher_classes
            WHERE teacher_id = current_setting('app.current_user_id', true)::uuid
        )
    );

-- Material assignments: visible to the class's teacher
CREATE POLICY teacher_isolation ON class_material_assignments
    USING (
        class_id IN (
            SELECT id FROM teacher_classes
            WHERE teacher_id = current_setting('app.current_user_id', true)::uuid
        )
    );

-- ============================================================
-- TRIGGERS
-- ============================================================
CREATE TRIGGER trg_teacher_classes_updated_at
    BEFORE UPDATE ON teacher_classes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
