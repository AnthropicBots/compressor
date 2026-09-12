#ifndef COMPRESSOR_H
#define COMPRESSOR_H

#include <string>
#include "HuffmanTree.h"

using namespace std;

class Compressor {
public:
    void compress(const string& inputFile, const string& outputFile);
};

#endif
