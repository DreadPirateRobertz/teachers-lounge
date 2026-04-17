"""Chemistry concept seed data for the global ltree knowledge graph (tl-mhd).

The ``concept_graph`` table encodes prerequisite relationships positionally:
a concept's ancestors in the ltree path are treated as its prerequisites. This
module exposes a hand-authored catalog of 52 chemistry concepts — 6
category nodes ("Organic Chemistry", "Stereochemistry", …) plus 46 leaf
topics — so that querying, e.g., the ancestors of
``chemistry.organic.stereochemistry.chirality`` yields the expected
prerequisite concepts ("Organic Chemistry", "Stereochemistry").
"""

from __future__ import annotations

from dataclasses import dataclass

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from ..orm import ConceptGraphNode


@dataclass(frozen=True)
class ChemistryConcept:
    """A single node in the chemistry seed catalog.

    Attributes:
        concept_id: Stable slug; used as the public identifier in endpoints.
        label: Human-readable title shown to students and in prereq-gap notes.
        path: ltree path; ancestors of this path are treated as prerequisites.
    """

    concept_id: str
    label: str
    path: str


SUBJECT = "chemistry"


# ── Curriculum ────────────────────────────────────────────────────────────────
# Category nodes appear before their children so ancestor queries resolve to
# real rows. Exactly 50 entries.

CHEMISTRY_CONCEPTS: tuple[ChemistryConcept, ...] = (
    # ── Category spine (6) ────────────────────────────────────────────────────
    ChemistryConcept("foundations_chem", "Foundations of Chemistry", "chemistry.foundations"),
    ChemistryConcept("general_chem", "General Chemistry", "chemistry.general"),
    ChemistryConcept("organic_chem", "Organic Chemistry", "chemistry.organic"),
    ChemistryConcept(
        "stereochemistry", "Stereochemistry", "chemistry.organic.stereochemistry"
    ),
    ChemistryConcept("organic_mechanisms", "Reaction Mechanisms", "chemistry.organic.mechanisms"),
    ChemistryConcept("analytical_chem", "Analytical Chemistry", "chemistry.analytical"),
    # ── Foundations (5) ───────────────────────────────────────────────────────
    ChemistryConcept(
        "atomic_structure", "Atomic Structure", "chemistry.foundations.atomic_structure"
    ),
    ChemistryConcept(
        "periodic_table", "Periodic Table", "chemistry.foundations.periodic_table"
    ),
    ChemistryConcept(
        "electron_configuration",
        "Electron Configuration",
        "chemistry.foundations.electron_configuration",
    ),
    ChemistryConcept(
        "chemical_bonding", "Chemical Bonding", "chemistry.foundations.chemical_bonding"
    ),
    ChemistryConcept(
        "vsepr_theory", "VSEPR Theory", "chemistry.foundations.vsepr_theory"
    ),
    # ── General chemistry (8) ─────────────────────────────────────────────────
    ChemistryConcept(
        "chemical_equations", "Chemical Equations", "chemistry.general.chemical_equations"
    ),
    ChemistryConcept(
        "stoichiometry", "Stoichiometry", "chemistry.general.stoichiometry"
    ),
    ChemistryConcept(
        "limiting_reagents", "Limiting Reagents", "chemistry.general.limiting_reagents"
    ),
    ChemistryConcept("gas_laws", "Gas Laws", "chemistry.general.gas_laws"),
    ChemistryConcept(
        "solutions_concentration",
        "Solutions and Concentration",
        "chemistry.general.solutions_and_concentration",
    ),
    ChemistryConcept(
        "thermochemistry", "Thermochemistry", "chemistry.general.thermochemistry"
    ),
    ChemistryConcept(
        "chemical_equilibrium", "Chemical Equilibrium", "chemistry.general.equilibrium"
    ),
    ChemistryConcept("acid_base", "Acid-Base Chemistry", "chemistry.general.acid_base"),
    # ── Organic foundations (6) ───────────────────────────────────────────────
    ChemistryConcept(
        "hybridization", "Hybridization", "chemistry.organic.structural.hybridization"
    ),
    ChemistryConcept("hydrocarbons", "Hydrocarbons", "chemistry.organic.foundations.hydrocarbons"),
    ChemistryConcept("alkanes", "Alkanes", "chemistry.organic.foundations.alkanes"),
    ChemistryConcept("alkenes", "Alkenes", "chemistry.organic.foundations.alkenes"),
    ChemistryConcept("alkynes", "Alkynes", "chemistry.organic.foundations.alkynes"),
    ChemistryConcept(
        "aromatic_compounds", "Aromatic Compounds", "chemistry.organic.foundations.aromatics"
    ),
    ChemistryConcept(
        "functional_groups",
        "Functional Groups",
        "chemistry.organic.foundations.functional_groups",
    ),
    # ── Organic structural (5) ────────────────────────────────────────────────
    ChemistryConcept(
        "iupac_nomenclature",
        "IUPAC Nomenclature",
        "chemistry.organic.structural.iupac_nomenclature",
    ),
    ChemistryConcept(
        "structural_isomers",
        "Structural Isomers",
        "chemistry.organic.structural.structural_isomers",
    ),
    ChemistryConcept(
        "conformational_analysis",
        "Conformational Analysis",
        "chemistry.organic.structural.conformational_analysis",
    ),
    ChemistryConcept(
        "resonance_structures",
        "Resonance Structures",
        "chemistry.organic.structural.resonance_structures",
    ),
    ChemistryConcept(
        "inductive_effects",
        "Inductive Effects",
        "chemistry.organic.structural.inductive_effects",
    ),
    # ── Stereochemistry (4 leaves; the category node is above) ────────────────
    ChemistryConcept(
        "chirality", "Chirality", "chemistry.organic.stereochemistry.chirality"
    ),
    ChemistryConcept(
        "r_s_configuration",
        "R/S Configuration",
        "chemistry.organic.stereochemistry.r_s_configuration",
    ),
    ChemistryConcept(
        "enantiomers", "Enantiomers", "chemistry.organic.stereochemistry.enantiomers"
    ),
    ChemistryConcept(
        "diastereomers", "Diastereomers", "chemistry.organic.stereochemistry.diastereomers"
    ),
    # ── Reaction mechanisms (5 leaves under mechanisms category) ──────────────
    ChemistryConcept(
        "sn1_reactions", "SN1 Reactions", "chemistry.organic.mechanisms.sn1_reactions"
    ),
    ChemistryConcept(
        "sn2_reactions", "SN2 Reactions", "chemistry.organic.mechanisms.sn2_reactions"
    ),
    ChemistryConcept(
        "e1_reactions", "E1 Reactions", "chemistry.organic.mechanisms.e1_reactions"
    ),
    ChemistryConcept(
        "e2_reactions", "E2 Reactions", "chemistry.organic.mechanisms.e2_reactions"
    ),
    ChemistryConcept(
        "electrophilic_addition",
        "Electrophilic Addition",
        "chemistry.organic.mechanisms.electrophilic_addition",
    ),
    # ── Functional-group chemistry (5) ────────────────────────────────────────
    ChemistryConcept("alcohols", "Alcohols", "chemistry.organic.reactions.alcohols"),
    ChemistryConcept(
        "aldehydes_ketones",
        "Aldehydes and Ketones",
        "chemistry.organic.reactions.aldehydes_ketones",
    ),
    ChemistryConcept(
        "carboxylic_acids",
        "Carboxylic Acids",
        "chemistry.organic.reactions.carboxylic_acids",
    ),
    ChemistryConcept(
        "esters_amides", "Esters and Amides", "chemistry.organic.reactions.esters_amides"
    ),
    ChemistryConcept("amines", "Amines", "chemistry.organic.reactions.amines"),
    # ── Advanced / named reactions (4) ────────────────────────────────────────
    ChemistryConcept(
        "aldol_condensation",
        "Aldol Condensation",
        "chemistry.organic.advanced.aldol_condensation",
    ),
    ChemistryConcept(
        "grignard_reactions",
        "Grignard Reactions",
        "chemistry.organic.advanced.grignard_reactions",
    ),
    ChemistryConcept(
        "diels_alder", "Diels-Alder Reaction", "chemistry.organic.advanced.diels_alder"
    ),
    ChemistryConcept(
        "retrosynthesis", "Retrosynthesis", "chemistry.organic.advanced.retrosynthesis"
    ),
    # ── Analytical methods (3) ────────────────────────────────────────────────
    ChemistryConcept(
        "ir_spectroscopy", "IR Spectroscopy", "chemistry.analytical.ir_spectroscopy"
    ),
    ChemistryConcept(
        "nmr_spectroscopy", "NMR Spectroscopy", "chemistry.analytical.nmr_spectroscopy"
    ),
    ChemistryConcept(
        "mass_spectrometry", "Mass Spectrometry", "chemistry.analytical.mass_spectrometry"
    ),
)


@dataclass
class SeedResult:
    """Outcome of applying :func:`seed_chemistry_concepts` to the graph.

    Attributes:
        inserted: Count of newly-inserted concept_graph rows.
        skipped:  Count of concepts already present (by concept_id).
    """

    inserted: int = 0
    skipped: int = 0


async def seed_chemistry_concepts(db: AsyncSession) -> SeedResult:
    """Insert the chemistry curriculum into ``concept_graph``.

    Idempotent on ``concept_id``: concepts already present are left untouched,
    so this is safe to run repeatedly (e.g. during bootstrap or after partial
    failures). The caller is responsible for committing the transaction.

    Args:
        db: Open async SQLAlchemy session.

    Returns:
        :class:`SeedResult` summarising inserted vs. skipped counts.
    """
    existing = await db.execute(select(ConceptGraphNode.concept_id))
    present: set[str] = {row for row in existing.scalars().all()}

    result = SeedResult()
    for spec in CHEMISTRY_CONCEPTS:
        if spec.concept_id in present:
            result.skipped += 1
            continue
        db.add(
            ConceptGraphNode(
                concept_id=spec.concept_id,
                label=spec.label,
                subject=SUBJECT,
                path=spec.path,
            )
        )
        result.inserted += 1

    await db.flush()
    return result
