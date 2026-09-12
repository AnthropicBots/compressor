#ifndef HUFFMANTREE_H
#define HUFFMANTREE_H

#include <map>
#include <queue>
#include <string>
#include "HuffmanNode.h"

using namespace std;

class HuffmanTree {
private:
    HuffmanNode* root;
    map<char, string> codes;
    map<char, int> freq;

    void generateCodes(HuffmanNode* node, string code);

public:
    HuffmanTree();

    void buildFrequency(const string& text);
    void setFrequency(const map<char,int>& f);   // ⭐ NEW
    void buildTree();
    void createCodes();

    map<char, string> getCodes();
    map<char, int> getFrequency();
    HuffmanNode* getRoot();
};

#endif
