// Copyright 2013-2014 Bowery, Inc.
#import "Panel.h"

@implementation Panel

- (BOOL)canBecomeKeyWindow;
{
    return YES; // Allow Search field to become the first responder
}

@end
