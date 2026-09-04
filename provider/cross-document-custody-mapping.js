'use strict';

function canonicalOriginalDocumentIdentity(uri) {
  return String(uri);
}

function canonicalVirtualDocumentIdentity({ name }) {
  return String(name);
}

class DocumentCustody {
  register(record) {
    return record;
  }

  resolve() {
    return undefined;
  }
}

function createDocumentMapping(specification) {
  return {
    translateOriginalRange(range) {
      return { ...range, document: specification.virtual?.identity };
    },
  };
}

module.exports = {
  canonicalOriginalDocumentIdentity,
  canonicalVirtualDocumentIdentity,
  createDocumentMapping,
  DocumentCustody,
};
