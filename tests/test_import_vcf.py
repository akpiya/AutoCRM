from __future__ import annotations

from pathlib import Path

from scripts.import_imessage_notion import parse_vcf


def test_parse_vcf_basic(tmp_path: Path) -> None:
    vcf = tmp_path / "sample.vcf"
    vcf.write_text(
        """BEGIN:VCARD
VERSION:3.0
FN:Jane Doe
TEL;TYPE=CELL:+1 555-123-4567
EMAIL;TYPE=INTERNET:jane@example.com
END:VCARD
BEGIN:VCARD
FN:Bob Smith
TEL:5559876543
END:VCARD
""",
        encoding="utf-8",
    )
    contacts = parse_vcf(vcf)
    assert len(contacts) == 2
    assert contacts[0].name == "Jane Doe"
    assert "+1 555-123-4567" in contacts[0].phones
    assert "jane@example.com" in contacts[0].emails
    assert contacts[1].name == "Bob Smith"
    assert contacts[1].phones == ("5559876543",)
