// Copyright 2013-2014 Bowery, Inc.
#define ARROW_WIDTH 12
#define ARROW_HEIGHT 8

@interface BackgroundView : NSView
{
    NSInteger _arrowX;
}

@property (nonatomic, assign) NSInteger arrowX;

@end
