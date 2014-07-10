//
//  BoweryAppDelegate.h
//  BoweryMenuApp
//
//  Created by Steve Kaliski on 7/10/14.
//  Copyright (c) 2014 Bowery. All rights reserved.
//

#import <Cocoa/Cocoa.h>

@class WebView;

@interface BoweryAppDelegate : NSObject <NSApplicationDelegate>

@property (weak) IBOutlet WebView *webView;
@property (strong, nonatomic) IBOutlet NSButton *refreshBtn;
@property (assign) IBOutlet NSWindow *window;

@end
