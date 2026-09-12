#ifndef DECOMPRESSOR_H
#define DECOMPRESSOR_H

#include <string>
#include <map>
#include "HuffmanTree.h"

using namespace std;

class Decompressor {
public:
    void decompress(const string& inputFile, const string& outputFile);
};

#endif
