describe("Storage Explorer spec", () => {
  beforeEach(() => {
    cy.viewport("macbook-16");
    cy.visit("/storage");
    cy.wait(500);
  });

  it("should retrieve correct buckets", () => {
    cy.get("h2").first().should("have.text", "Bucket - test-bucket");
    cy.getTestEl("bucket-select").within(() => cy.get("button").click());

    const expectedBuckets = ["test-bucket"];

    cy.getTestEl("bucket-select-options")
      .find("li")
      .should("have.length", expectedBuckets.length)
      .each(($li, i) => {
        // Assert that each list item contains the expected text
        expect(expectedBuckets).to.include($li.text());
      });
  });

  it("should load files of first bucket", () => {
    cy.intercept("/api/storage?action=list-files*").as("getFiles");

    cy.wait("@getFiles");

    cy.get('button[title="test-bucket"]').should("exist");
  });

  it("should upload a file to bucket", () => {
    cy.intercept("/api/storage?action=write-file*").as("writeFile");
    cy.fixture("photo.jpg").then((fileContent) => {
      // Use cy.get() to select the file input element and upload the file
      cy.getTestEl("file-upload").then((el) => {
        // Upload the file to the input element
        const testFile = new File([fileContent], "storage-test-photo.jpg", {
          type: "image/jpeg",
        });
        const dataTransfer = new DataTransfer();
        dataTransfer.items.add(testFile);
        const fileInput = el[0];
        // @ts-ignore
        fileInput.files = dataTransfer.files;
        // Trigger a 'change' event on the input element
        cy.wrap(fileInput).trigger("change", { force: true });
      });
    });

    cy.wait("@writeFile");

    cy.get('[data-chonky-file-id="storage-test-photo.jpg"]', {
      timeout: 5000,
    }).should("exist");
  });

  it("should remove a file from bucket", () => {
    cy.get('[data-chonky-file-id="storage-test-photo.jpg"]').type("{del}");

    cy.get('[data-chonky-file-id="storage-test-photo.jpg"]').should(
      "not.exist"
    );
  });
});
