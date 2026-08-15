package sqlite

import (
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestErrorTranslator(t *testing.T) {
	// This is the DSN of the in-memory SQLite database for these tests.
	const InMemoryDSN = "file:testdatabase?mode=memory&cache=shared"

	// This is the example object for testing the unique constraint error
	type Article struct {
		ArticleNumber string `gorm:"unique"`
	}

	db, err := gorm.Open(&Dialector{DSN: InMemoryDSN}, &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true})

	if err != nil {
		t.Errorf("Expected Open to succeed; got error: %v", err)
	}
	if db == nil {
		t.Errorf("Expected db to be non-nil.")
	}

	err = db.AutoMigrate(&Article{})
	if err != nil {
		t.Errorf("Expected to migrate database models to succeed: %v", err)
	}

	err = db.Create(&Article{ArticleNumber: "A00000XX"}).Error
	if err != nil {
		t.Errorf("Expected first create to succeed: %v", err)
	}

	err = db.Create(&Article{ArticleNumber: "A00000XX"}).Error
	if err == nil {
		t.Errorf("Expected second create to fail.")
	}

	if err != gorm.ErrDuplicatedKey {
		t.Errorf("Expected error from second create to be gorm.ErrDuplicatedKey: %v", err)
	}
}

func TestErrorTranslatorCheckConstraint(t *testing.T) {
	// A DSN of its own, so the shared in-memory cache of the other tests is untouched.
	const InMemoryDSN = "file:testdatabase_check?mode=memory&cache=shared"

	// Price carries a CHECK constraint, which SQLite reports as extended code 275.
	type Product struct {
		Name  string
		Price int `gorm:"check:price_positive,price > 0"`
	}

	db, err := gorm.Open(&Dialector{DSN: InMemoryDSN}, &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true})

	if err != nil {
		t.Fatalf("Expected Open to succeed; got error: %v", err)
	}

	if err := db.AutoMigrate(&Product{}); err != nil {
		t.Fatalf("Expected to migrate database models to succeed: %v", err)
	}

	// Without this the failing insert below could fail for an unrelated reason and
	// the test would still look green.
	var ddl string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'products'").Scan(&ddl).Error; err != nil {
		t.Fatalf("Expected to read the table DDL: %v", err)
	}
	if !strings.Contains(strings.ToUpper(ddl), "CHECK") {
		t.Fatalf("Expected the products table to carry a CHECK constraint; got: %s", ddl)
	}

	if err := db.Create(&Product{Name: "valid", Price: 10}).Error; err != nil {
		t.Errorf("Expected a create satisfying the check to succeed: %v", err)
	}

	err = db.Create(&Product{Name: "invalid", Price: -1}).Error
	if err == nil {
		t.Fatalf("Expected a create violating the check to fail.")
	}

	if err != gorm.ErrCheckConstraintViolated {
		t.Errorf("Expected error from the violating create to be gorm.ErrCheckConstraintViolated: %v", err)
	}
}
